package wm

import (
	"context"
	"encoding/binary"
	"math"
	"slices"

	"github.com/grunclepug/stream/internal/config"
	"github.com/grunclepug/stream/internal/proto"
)

// Input protocol versions: the first with the done events, which river 0.5.0
// advertises. Both globals are optional: without them swm leaves input
// devices as river sets them up.
const (
	inputManagerVersion   = 2
	libinputConfigVersion = 2
)

// inputDevice is an input device as river_input_manager_v1 reports it. swm
// sets the key repeat of keyboards.
type inputDevice struct {
	proto.RiverInputDeviceV1Stub

	wm   *WM
	obj  proto.RiverInputDeviceV1
	typ  uint32
	name string

	ready       bool  // its type and name are known
	rate, delay int32 // the key repeat last sent, if sentRepeat
	sentRepeat  bool
	libinput    *libinputDevice // nil if it isn't a libinput device
}

// libinputDevice is the libinput side of an input device, whose options
// input lines set.
type libinputDevice struct {
	proto.RiverLibinputDeviceV1Stub

	wm  *WM
	obj proto.RiverLibinputDeviceV1
	dev *inputDevice // nil until river names it

	tapFingers int32 // more than 0 makes a pointer a touchpad
	ready      bool  // the first done event came

	tap, drag, naturalScroll, dwt, leftHanded, middleEmulation option
	accelProfile                                               option
	accelSpeed                                                 speedOption
}

// option is a libinput option the device supports once river has sent its
// default, with the value it has now.
type option struct {
	known    bool
	def, cur uint32
}

type speedOption struct {
	known    bool
	def, cur float64
}

// libinputResult logs why river refused to set an option.
type libinputResult struct {
	proto.RiverLibinputResultV1Stub
	wm           *WM
	device, name string
}

func (wm *WM) HandleRiverInputManagerV1InputDevice(ctx context.Context, id proto.RiverInputDeviceV1) {
	d := &inputDevice{wm: wm, obj: id}
	id.SetUserData(d)
	wm.inputs = append(wm.inputs, d)
}

func (wm *WM) HandleRiverLibinputConfigV1LibinputDevice(ctx context.Context, id proto.RiverLibinputDeviceV1) {
	d := &libinputDevice{wm: wm, obj: id}
	id.SetUserData(d)
}

// applyInput brings every input device in line with the config.
func (wm *WM) applyInput() {
	for _, d := range wm.inputs {
		d.apply()
	}
}

// typeName is the device's type as input lines name it.
func (d *inputDevice) typeName() string {
	switch d.typ {
	case proto.RiverInputDeviceV1TypeKeyboard:
		return "keyboard"
	case proto.RiverInputDeviceV1TypePointer:
		if d.libinput != nil && d.libinput.tapFingers > 0 {
			return "touchpad"
		}
		return "pointer"
	case proto.RiverInputDeviceV1TypeTouch:
		return "touch"
	case proto.RiverInputDeviceV1TypeTablet:
		return "tablet"
	}
	return ""
}

func (d *inputDevice) HandleRiverInputDeviceV1Type(ctx context.Context, typ uint32) {
	d.typ = typ
}

func (d *inputDevice) HandleRiverInputDeviceV1Name(ctx context.Context, name string) {
	d.name = name
}

func (d *inputDevice) HandleRiverInputDeviceV1Done(ctx context.Context) {
	if d.ready {
		return
	}
	d.ready = true
	d.wm.log.Debug("input device", "name", d.name)
	d.apply()
}

func (d *inputDevice) HandleRiverInputDeviceV1Removed(ctx context.Context) {
	d.wm.inputs = slices.DeleteFunc(d.wm.inputs, func(o *inputDevice) bool { return o == d })
	d.obj.Destroy()
}

// apply sets the key repeat and libinput options the config gives the
// device, once river has said what it is.
func (d *inputDevice) apply() {
	if !d.ready {
		return
	}
	cfg := d.wm.cfg
	if d.typ == proto.RiverInputDeviceV1TypeKeyboard {
		rate, delay := int32(cfg.RepeatRate), int32(cfg.RepeatDelay)
		if !d.sentRepeat || rate != d.rate || delay != d.delay {
			d.obj.SetRepeatInfo(rate, delay)
			d.rate, d.delay, d.sentRepeat = rate, delay, true
		}
	}
	if l := d.libinput; l != nil && l.ready {
		l.apply(cfg.InputFor(d.typeName(), d.name))
	}
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1InputDevice(ctx context.Context, device proto.RiverInputDeviceV1) {
	d, ok := device.UserData().(*inputDevice)
	if !ok {
		return // swm never saw it: river sent it before the input manager's
	}
	l.dev, d.libinput = d, l
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1Done(ctx context.Context) {
	if l.ready {
		return
	}
	l.ready = true
	if l.dev != nil {
		l.wm.log.Debug("libinput device", "name", l.dev.name, "type", l.dev.typeName())
		l.dev.apply()
	}
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1Removed(ctx context.Context) {
	if l.dev != nil {
		l.dev.libinput = nil
	}
	l.obj.Destroy()
}

// apply sets each option in to its value, or back to the device's default
// where in leaves it out. Options the device lacks are left alone.
func (l *libinputDevice) apply(in config.Input) {
	l.setSwitch("tap", &l.tap, in.Tap, l.obj.SetTap)
	l.setSwitch("drag", &l.drag, in.Drag, l.obj.SetDrag)
	l.setSwitch("natural-scroll", &l.naturalScroll, in.NaturalScroll, l.obj.SetNaturalScroll)
	l.setSwitch("dwt", &l.dwt, in.DWT, l.obj.SetDwt)
	l.setSwitch("left-handed", &l.leftHanded, in.LeftHanded, l.obj.SetLeftHanded)
	l.setSwitch("middle-emulation", &l.middleEmulation, in.MiddleEmulation, l.obj.SetMiddleEmulation)

	profile := l.accelProfile.def
	switch in.AccelProfile {
	case config.AccelFlat:
		profile = proto.RiverLibinputDeviceV1AccelProfileFlat
	case config.AccelAdaptive:
		profile = proto.RiverLibinputDeviceV1AccelProfileAdaptive
	}
	l.set("accel-profile", &l.accelProfile, profile, l.obj.SetAccelProfile)

	if s := &l.accelSpeed; s.known {
		speed := s.def
		if in.AccelSpeed != nil {
			speed = *in.AccelSpeed
		}
		if speed != s.cur {
			l.result("accel-speed", l.obj.SetAccelSpeed(encodeDouble(speed)))
			s.cur = speed
		}
	}
}

// setSwitch sets an on/off option. Every one of them has disabled 0 and
// enabled 1.
func (l *libinputDevice) setSwitch(name string, o *option, sw config.Switch, set func(uint32) proto.RiverLibinputResultV1) {
	want := o.def
	switch sw {
	case config.SwitchOn:
		want = 1
	case config.SwitchOff:
		want = 0
	}
	l.set(name, o, want, set)
}

func (l *libinputDevice) set(name string, o *option, want uint32, set func(uint32) proto.RiverLibinputResultV1) {
	if !o.known || o.cur == want {
		return
	}
	l.result(name, set(want))
	o.cur = want // river confirms with a current event
}

func (l *libinputDevice) result(name string, r proto.RiverLibinputResultV1) {
	device := ""
	if l.dev != nil {
		device = l.dev.name
	}
	r.SetUserData(&libinputResult{wm: l.wm, device: device, name: name})
	l.wm.log.Debug("input option", "device", device, "option", name)
}

func (r *libinputResult) HandleRiverLibinputResultV1Unsupported(ctx context.Context) {
	r.wm.log.Warn("input device doesn't support the option", "device", r.device, "option", r.name)
}

func (r *libinputResult) HandleRiverLibinputResultV1Invalid(ctx context.Context) {
	r.wm.log.Warn("input option refused as invalid", "device", r.device, "option", r.name)
}

// encodeDouble and decodeDouble convert the doubles libinput options carry,
// which the protocol sends as native-endian IEEE 754 arrays.
func encodeDouble(f float64) []byte {
	return binary.NativeEndian.AppendUint64(nil, math.Float64bits(f))
}

func decodeDouble(b []byte) (float64, bool) {
	if len(b) != 8 {
		return 0, false
	}
	return math.Float64frombits(binary.NativeEndian.Uint64(b)), true
}

// The events below record what each option is. A default event means the
// device supports the option; the current events follow it, and come again
// on every change.

func (l *libinputDevice) HandleRiverLibinputDeviceV1TapSupport(ctx context.Context, fingers int32) {
	l.tapFingers = fingers
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1TapDefault(ctx context.Context, state uint32) {
	l.tap.known, l.tap.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1TapCurrent(ctx context.Context, state uint32) {
	l.tap.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1DragDefault(ctx context.Context, state uint32) {
	l.drag.known, l.drag.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1DragCurrent(ctx context.Context, state uint32) {
	l.drag.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1NaturalScrollDefault(ctx context.Context, state uint32) {
	l.naturalScroll.known, l.naturalScroll.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1NaturalScrollCurrent(ctx context.Context, state uint32) {
	l.naturalScroll.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1DwtDefault(ctx context.Context, state uint32) {
	l.dwt.known, l.dwt.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1DwtCurrent(ctx context.Context, state uint32) {
	l.dwt.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1LeftHandedDefault(ctx context.Context, state uint32) {
	l.leftHanded.known, l.leftHanded.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1LeftHandedCurrent(ctx context.Context, state uint32) {
	l.leftHanded.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1MiddleEmulationDefault(ctx context.Context, state uint32) {
	l.middleEmulation.known, l.middleEmulation.def = true, state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1MiddleEmulationCurrent(ctx context.Context, state uint32) {
	l.middleEmulation.cur = state
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1AccelProfileDefault(ctx context.Context, profile uint32) {
	l.accelProfile.known, l.accelProfile.def = true, profile
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1AccelProfileCurrent(ctx context.Context, profile uint32) {
	l.accelProfile.cur = profile
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1AccelSpeedDefault(ctx context.Context, speed []byte) {
	if f, ok := decodeDouble(speed); ok {
		l.accelSpeed.known, l.accelSpeed.def = true, f
	}
}

func (l *libinputDevice) HandleRiverLibinputDeviceV1AccelSpeedCurrent(ctx context.Context, speed []byte) {
	if f, ok := decodeDouble(speed); ok {
		l.accelSpeed.cur = f
	}
}
