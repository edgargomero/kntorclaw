package events

import (
	"strings"
	"testing"
)

func TestDeviceEvent_FormatMessage_Add(t *testing.T) {
	ev := &DeviceEvent{
		Action:       ActionAdd,
		Kind:         KindUSB,
		DeviceID:     "1-2",
		Vendor:       "Arduino",
		Product:      "Mega 2560",
		Serial:       "SN123456",
		Capabilities: "Serial, HID",
	}

	msg := ev.FormatMessage()

	if !strings.Contains(msg, "Connected") {
		t.Error("FormatMessage for Add should contain 'Connected'")
	}
	if !strings.Contains(msg, "usb") {
		t.Error("FormatMessage should contain device kind")
	}
	if !strings.Contains(msg, "Arduino") {
		t.Error("FormatMessage should contain vendor")
	}
	if !strings.Contains(msg, "Mega 2560") {
		t.Error("FormatMessage should contain product")
	}
	if !strings.Contains(msg, "Serial, HID") {
		t.Error("FormatMessage should contain capabilities")
	}
	if !strings.Contains(msg, "SN123456") {
		t.Error("FormatMessage should contain serial number")
	}
}

func TestDeviceEvent_FormatMessage_Remove(t *testing.T) {
	ev := &DeviceEvent{
		Action:  ActionRemove,
		Kind:    KindUSB,
		Vendor:  "Logitech",
		Product: "Mouse",
	}

	msg := ev.FormatMessage()

	if !strings.Contains(msg, "Disconnected") {
		t.Error("FormatMessage for Remove should contain 'Disconnected'")
	}
	if !strings.Contains(msg, "Logitech") {
		t.Error("FormatMessage should contain vendor")
	}
}

func TestDeviceEvent_FormatMessage_NoOptionalFields(t *testing.T) {
	ev := &DeviceEvent{
		Action:  ActionAdd,
		Kind:    KindGeneric,
		Vendor:  "Unknown",
		Product: "Device",
	}

	msg := ev.FormatMessage()

	// Should NOT contain Capabilities or Serial lines
	if strings.Contains(msg, "Capabilities:") {
		t.Error("FormatMessage should not include Capabilities line when empty")
	}
	if strings.Contains(msg, "Serial:") {
		t.Error("FormatMessage should not include Serial line when empty")
	}
}

func TestAction_Constants(t *testing.T) {
	if ActionAdd != "add" {
		t.Errorf("ActionAdd = %q, want %q", ActionAdd, "add")
	}
	if ActionRemove != "remove" {
		t.Errorf("ActionRemove = %q, want %q", ActionRemove, "remove")
	}
	if ActionChange != "change" {
		t.Errorf("ActionChange = %q, want %q", ActionChange, "change")
	}
}

func TestKind_Constants(t *testing.T) {
	if KindUSB != "usb" {
		t.Errorf("KindUSB = %q, want %q", KindUSB, "usb")
	}
	if KindBluetooth != "bluetooth" {
		t.Errorf("KindBluetooth = %q, want %q", KindBluetooth, "bluetooth")
	}
	if KindPCI != "pci" {
		t.Errorf("KindPCI = %q, want %q", KindPCI, "pci")
	}
	if KindGeneric != "generic" {
		t.Errorf("KindGeneric = %q, want %q", KindGeneric, "generic")
	}
}
