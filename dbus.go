package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/jeremija/wl-gammarelay/display"
	"github.com/peer-calls/log"
)

const (
	dbusServiceName   = "rs.wl-gammarelay"
	dbusObjectPath    = "/"
	dbusInterfaceName = "rs.wl.gammarelay"
	introspectable    = "org.freedesktop.DBus.Introspectable"

	propTemperature = "Temperature"
	propBrightness  = "Brightness"
)

type srv struct {
	mu    sync.Mutex
	props *prop.Properties
}

func (s *srv) UpdateTemperature(temperature int16) (err *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, propTemperature)
	if err != nil {
		return err
	}

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

	return s.props.Set(dbusInterfaceName, propTemperature, dbus.MakeVariant(value))
}

func (s *srv) UpdateBrightness(brightness float64) (err *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, propBrightness)
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
		return dbus.MakeFailedError(fmt.Errorf("value is not double: %T", v.Value()))
	}

	value += brightness

	return s.props.Set(dbusInterfaceName, propBrightness, dbus.MakeVariant(value))
}

type Display interface {
	SetColor(context.Context, display.ColorParams) error
}

func NewDBus(ctx context.Context, logger log.Logger, disp Display) (*dbus.Conn, error) {
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
				propTemperature: {
					Value:    uint16(data.temp),
					Writable: true,
					Emit:     prop.EmitTrue,
					Callback: func(c *prop.Change) *dbus.Error {
						temp, _ := c.Value.(uint16)

						err := disp.SetColor(ctx, display.ColorParams{
							Temperature: int(temp),
							Brightness:  float32(data.brightness),
						})
						if err != nil {
							logger.Error("Failed to set temperature", err, nil)
							return dbus.MakeFailedError(fmt.Errorf("failed to set color: %w", err))
						}

						data.temp = temp

						return nil
					},
				},
				propBrightness: {
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
							logger.Error("Failed to set brightness", err, nil)
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

type Subscriber struct {
	*dbus.Conn
}

func NewSubscriber(ctx context.Context) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to session bus:", err)
		os.Exit(1)
	}
	defer conn.Close()

	status := struct {
		temperature uint16
		brightness  float64
	}{
		temperature: 6500,
		brightness:  1.0,
	}

	obj := conn.Object(dbusServiceName, dbusObjectPath)

	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"eavesdrop='true',type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='"+dbusObjectPath+"'")
	if call.Err != nil {
		fmt.Fprintln(os.Stderr, "Failed to add match:", call.Err)
		os.Exit(1)
	}

	if v, err := obj.GetProperty(dbusInterfaceName + "." + propTemperature); err == nil {
		status.temperature = v.Value().(uint16)
	} else {
		fmt.Println(err)
	}

	if v, err := obj.GetProperty(dbusInterfaceName + "." + propBrightness); err == nil {
		status.brightness = v.Value().(float64)
	}

	fmt.Fprintf(os.Stdout, "%d %s\n",
		status.temperature, strconv.FormatFloat(status.brightness, 'f', 2, 64))

	c := make(chan *dbus.Message, 100)
	conn.Eavesdrop(c)

	for {
		select {
		case <-ctx.Done():
			return nil
		case v, ok := <-c:
			if !ok {
				return nil
			}
			if iface := v.Body[0]; iface != dbusInterfaceName {
				continue
			}

			if len(v.Body) < 2 {
				continue
			}

			m, ok := v.Body[1].(map[string]dbus.Variant)
			if !ok {
				continue
			}

			shouldPrint := false

			temperature, ok := m["Temperature"]
			if ok {
				if v, ok := temperature.Value().(uint16); ok {
					status.temperature = v
					shouldPrint = true
				}
			}

			brightness, ok := m["Brightness"]
			if ok {
				if v, ok := brightness.Value().(float64); ok {
					status.brightness = v
					shouldPrint = true
				}
			}

			if shouldPrint {
				fmt.Fprintf(os.Stdout, "%d %s\n",
					status.temperature, strconv.FormatFloat(status.brightness, 'f', 2, 64))
			}
		}
	}
}
