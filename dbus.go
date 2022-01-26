package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/jeremija/wl-gammarelay/display"
)

const (
	dbusServiceName   = "rs.wl-gammarelay"
	dbusObjectPath    = "/"
	dbusInterfaceName = "rs.wl.gammarelay"
	introspectable    = "org.freedesktop.DBus.Introspectable"
)

type srv struct {
	mu    sync.Mutex
	props *prop.Properties
}

func (s *srv) UpdateTemperature(temperature int16) (err *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, "Temperature")
	if err != nil {
		return err
	}

	fmt.Printf("Temp: %s %v %T\n", v.Value(), v.Value(), v.Value())

	var value uint16

	switch t := v.Value().(type) {
	case uint16:
		value = t
	case *uint16:
		value = *t
	default:
		return dbus.MakeFailedError(fmt.Errorf("value is not int16: %T", v.Value()))
	}

	value = uint16(int16(value) + temperature)

	fmt.Printf("test: %T %T\n", value, dbus.MakeVariant(value).Value())

	return s.props.Set(dbusInterfaceName, "Temperature", dbus.MakeVariant(value))
}

func (s *srv) UpdateBrightness(brightness float64) (err *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, "Brightness")
	if err != nil {
		return err
	}

	var value float64

	switch t := v.Value().(type) {
	case float64:
		value = t
	case *float64:
		value = *t
	default:
		return dbus.MakeFailedError(fmt.Errorf("value is not int16: %T", v.Value()))
	}

	value += brightness

	fmt.Println("setting brightness", value)

	return s.props.Set(dbusInterfaceName, "Brightness", dbus.MakeVariant(value))
}

type Display interface {
	SetColor(context.Context, display.ColorParams) error
}

func NewDBus(ctx context.Context, disp Display) (*dbus.Conn, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("falied to connect to dbus: %w", err)
	}

	init := func() error {
		reply, err := conn.RequestName(dbusServiceName, dbus.NameFlagDoNotQueue)
		if err != nil {
			return fmt.Errorf("failed to request name: %w", err)
		}

		if reply != dbus.RequestNameReplyPrimaryOwner {
			return fmt.Errorf("name already taken")
		}

		data := struct {
			temp       uint16
			brightness float64
		}{
			temp:       6500,
			brightness: 1,
		}

		propsSpec := map[string]map[string]*prop.Prop{
			dbusInterfaceName: {
				"Temperature": {
					Value:    uint16(data.temp),
					Writable: true,
					Emit:     prop.EmitTrue,
					Callback: func(c *prop.Change) *dbus.Error {
						fmt.Println("Temperature callback")
						temp, _ := c.Value.(uint16)

						fmt.Println("Calling SetColor")
						err := disp.SetColor(ctx, display.ColorParams{
							Temperature: int(temp),
							Brightness:  float32(data.brightness),
						})
						if err != nil {
							fmt.Println("Failed to set color")
							return dbus.MakeFailedError(fmt.Errorf("failed to set color: %w", err))
						}

						data.temp = temp

						return nil
					},
				},
				"Brightness": {
					Value:    float64(data.brightness),
					Writable: true,
					Emit:     prop.EmitTrue,
					Callback: func(c *prop.Change) *dbus.Error {
						bri, _ := c.Value.(float64)

						err := disp.SetColor(ctx, display.ColorParams{
							Temperature: int(data.temp),
							Brightness:  float32(bri),
						})
						if err != nil {
							fmt.Println("Failed to set color")
							return dbus.MakeFailedError(fmt.Errorf("failed to set color: %w", err))
						}

						data.brightness = bri

						return nil
					},
				},
			},
		}

		props, err := prop.Export(conn, dbusObjectPath, propsSpec)
		if err != nil {
			return fmt.Errorf("export propsSpec failed: %w", err)
		}

		service := &srv{
			props: props,
		}

		if err := conn.Export(service, dbusObjectPath, dbusInterfaceName); err != nil {
			return fmt.Errorf("failed to register interface: %w", err)
		}

		n := &introspect.Node{
			Name: dbusObjectPath,
			Interfaces: []introspect.Interface{
				introspect.IntrospectData,
				prop.IntrospectData,
				{
					Name:       dbusInterfaceName,
					Methods:    introspect.Methods(service),
					Properties: props.Introspection(dbusInterfaceName),
				},
			},
		}

		if err = conn.Export(
			introspect.NewIntrospectable(n),
			dbusObjectPath,
			"org.freedesktop.DBus.Introspectable",
		); err != nil {
			return fmt.Errorf("export introspectable failed: %w", err)
		}

		return nil
	}

	if err := init(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to register dbus: %w", err)
	}

	return conn, nil
}
