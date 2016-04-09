package gpio

type Direction uint8
type State uint8
type Pull uint8

type Pin struct {
	Gpio Gpio
	No   int
}

type Gpio interface {
	// Get a Pin from this Gpio
	Pin(int) Pin
	TogglePin(int)
	PinMode(int, Direction)
	WritePin(int, State)
	ReadPin(int)State
	PullMode(int, Pull)
	Close()error
}

// Pin direction, a pin can be set in Input or Output mode
const (
	Input Direction = iota
	Output
)

// State of pin, High / Low
const (
	Low State = iota
	High
)

// Pull Up / Down / Off
const (
	PullOff Pull = iota
	PullDown
	PullUp
)



// Set pin as Input
func (p Pin) Input() {
	p.Mode(Input)
}

// Set pin as Output
func (p Pin) Output() {
	p.Mode(Output)
}

// Set pin High
func (p Pin) High() {
	p.Write(High)
}

// Set pin Low
func (p Pin) Low() {
	p.Write(Low)
}

// Toggle pin state
func (p Pin) Toggle() {
	p.Gpio.TogglePin(p.No)
}

// Set pin Direction
func (p Pin) Mode(dir Direction) {
	p.Gpio.PinMode(p.No, dir)
}

// Set pin state (high/low)
func (p Pin) Write(state State) {
	p.Gpio.WritePin(p.No, state)
}

// Read pin state (high/low)
func (p Pin) Read() State {
	return p.Gpio.ReadPin(p.No)
}

// Set a given pull up/down mode
func (p Pin) Pull(pull Pull) {
	p.Gpio.PullMode(p.No, pull)
}

// Pull up pin
func (p Pin) PullUp() {
	p.Pull(PullUp)
}

// Pull down pin
func (p Pin) PullDown() {
	p.Pull(PullDown)
}

// Disable pullup/down on pin
func (p Pin) PullOff() {
	p.Pull(PullOff)
}
