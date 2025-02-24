package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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

func (s *srv) UpdateTemperature(temperature int16) (uint16, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, propTemperature)
	if err != nil {
		return 0, err
	}

	value, ok := v.Value().(uint16)
	if !ok {
		return value, dbus.MakeFailedError(fmt.Errorf("value is not uint16: %T", v.Value()))
	}

	value = uint16(int16(value) + temperature)

	err = s.props.Set(dbusInterfaceName, propTemperature, dbus.MakeVariant(value))
	if err != nil {
		return 0, err
	}

	return value, nil
}

func (s *srv) UpdateBrightness(brightness float64) (float64, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.props.Get(dbusInterfaceName, propBrightness)
	if err != nil {
		return 0, err
	}

	value, ok := v.Value().(float64)
	if !ok {
		return 0, dbus.MakeFailedError(fmt.Errorf("value is not double: %T", v.Value()))
	}

	value += brightness

	err = s.props.Set(dbusInterfaceName, propBrightness, dbus.MakeVariant(value))
	if err != nil {
		return 0, err
	}

	return value, nil
}

type Display interface {
	SetColor(context.Context, display.ColorParams) error
}

type DBus struct {
	conn   *dbus.Conn
	logger log.Logger
}

func NewDBus(logger log.Logger) (*DBus, error) {
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

		return nil
	}

	if err := init(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to register dbus: %w", err)
	}

	return &DBus{
		conn:   conn,
		logger: logger,
	}, nil
}

func (d *DBus) RegisterDisplayService(ctx context.Context, disp Display) error {
	logger := d.logger
	conn := d.conn

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

func (d *DBus) Close() error {
	return d.conn.Close()
}

type Subscriber struct {
	*dbus.Conn
}

type state struct {
	temperature uint16
	brightness  float64
}

func (s state) Format(sb strings.Builder, props []string) string {
	sb.Reset()

	for _, prop := range props {
		switch prop {
		case propTemperature:
			sb.WriteString(strconv.Itoa(int(s.temperature)))
		case propBrightness:
			sb.WriteString(strconv.FormatFloat(s.brightness, 'f', 2, 64))
		}

		sb.WriteString(" ")
	}

	return strings.TrimSuffix(sb.String(), " ")
}

func NewSubscriber(ctx context.Context, props []string) error {
	propsSet := make(map[string]struct{}, len(props))
	for _, p := range props {
		propsSet[p] = struct{}{}
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to session bus:", err)
		os.Exit(1)
	}
	defer conn.Close()

	status := state{
		temperature: 6500,
		brightness:  1.0,
	}

	obj := conn.Object(dbusServiceName, dbusObjectPath)

	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',path='"+dbusObjectPath+"'")
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

	var sb strings.Builder

	fmt.Fprintf(os.Stdout, status.Format(sb, props)+"\n")

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

			if _, ok := propsSet[propTemperature]; ok {
				temperature, ok := m[propTemperature]
				if ok {
					if v, ok := temperature.Value().(uint16); ok {
						status.temperature = v
						shouldPrint = true
					}
				}
			}

			if _, ok := propsSet[propBrightness]; ok {
				brightness, ok := m[propBrightness]
				if ok {
					if v, ok := brightness.Value().(float64); ok {
						status.brightness = v
						shouldPrint = true
					}
				}
			}

			if shouldPrint {
				fmt.Fprintf(os.Stdout, status.Format(sb, props)+"\n")
			}
		}
	}
}
