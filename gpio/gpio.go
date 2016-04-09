package gpio
// Based on https://github.com/stianeikeland/go-rpio
import (
	"sync"
	"unsafe"
	"syscall"
	"os"
	"encoding/binary"
	"bytes"
	"reflect"
	"time"
)
// Memory offsets for gpio, see the spec for more details
const (
	bcm2835Base = 0x20000000
	pi1GPIOBase = bcm2835Base + 0x200000
	memLength = 4096

	pinMask uint32 = 7 // 0b111 - pinmode is 3 bits
)

type gpio struct {
	memlock sync.Mutex
	mem     []uint32
	mem8    []uint8
}

func (g *gpio)Pin(n int) Pin {
	return Pin{Gpio:g, No:n}
}
func (g *gpio)Close() (error) {
	g.memlock.Lock()
	defer g.memlock.Unlock()
	return syscall.Munmap(g.mem8)
}

func OpenWith(mem, ranges string) (g Gpio, err error) {
	var file *os.File
	var base int64
	gpio := &gpio{}
	g = gpio
	// Open fd for rw mem access; try gpiomem first
	file, err = os.OpenFile(mem, os.O_RDWR | os.O_SYNC, 0)
	base = getGPIOBase(ranges)

	if err != nil {
		return
	}

	// FD can be closed after memory mapping
	defer file.Close()

	gpio.memlock.Lock()
	defer gpio.memlock.Unlock()

	// Memory map GPIO registers to byte array
	gpio.mem8, err = syscall.Mmap(
		int(file.Fd()),
		base,
		memLength,
		syscall.PROT_READ | syscall.PROT_WRITE,
		syscall.MAP_SHARED)

	if err != nil {
		return
	}

	// Convert mapped byte memory to unsafe []uint32 pointer, adjust length as needed
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&gpio.mem8))
	header.Len /= (32 / 8) // (32 bit = 4 bytes)
	header.Cap /= (32 / 8)

	gpio.mem = *(*[]uint32)(unsafe.Pointer(&header))

	return
}

// Open and memory map GPIO memory range from /dev/mem .
// Some reflection magic is used to convert it to a unsafe []uint32 pointer
func OpenDefault() (g Gpio, e error) {
	g, e = OpenWith("/dev/mem", "/proc/device-tree/soc/ranges")
	return
}


// Read /proc/device-tree/soc/ranges and determine the base address.
// Use the default Raspberry Pi 1 base address if this fails.
func getGPIOBase(rangesFile string) (base int64) {
	base = pi1GPIOBase
	ranges, err := os.Open(rangesFile)
	defer ranges.Close()
	if err != nil {
		return
	}
	b := make([]byte, 4)
	n, err := ranges.ReadAt(b, 4)
	if n != 4 || err != nil {
		return
	}
	buf := bytes.NewReader(b)
	var out uint32
	err = binary.Read(buf, binary.BigEndian, &out)
	if err != nil {
		return
	}
	return int64(out + 0x200000)
}


// PinMode sets the direction of a given pin (Input or Output)
func (g *gpio)PinMode(pin int, direction Direction) {

	// Pin fsel register, 0 or 1 depending on bank
	fsel := uint8(pin) / 10
	shift := (uint8(pin) % 10) * 3

	g.memlock.Lock()
	defer g.memlock.Unlock()

	if direction == Input {
		g.mem[fsel] = g.mem[fsel] &^ (pinMask << shift)
	} else {
		g.mem[fsel] = (g.mem[fsel] &^ (pinMask << shift)) | (1 << shift)
	}

}

// WritePin sets a given pin High or Low
// by setting the clear or set registers respectively
func (g *gpio)WritePin(pin int, state State) {

	p := uint8(pin)

	// Clear register, 10 / 11 depending on bank
	// Set register, 7 / 8 depending on bank
	clearReg := p / 32 + 10
	setReg := p / 32 + 7

	g.memlock.Lock()
	defer g.memlock.Unlock()

	if state == Low {
		g.mem[clearReg] = 1 << (p & 31)
	} else {
		g.mem[setReg] = 1 << (p & 31)
	}

}

// Read the state of a pin
func (g*gpio)ReadPin(pin int) State {
	// Input level register offset (13 / 14 depending on bank)
	levelReg := uint8(pin) / 32 + 13

	if (g.mem[levelReg] & (1 << uint8(pin))) != 0 {
		return High
	}

	return Low
}

// Toggle a pin state (high -> low -> high)
// TODO: probably possible to do this much faster without read
func (g*gpio)TogglePin(pin int) {
	switch g.ReadPin(pin) {
	case Low:
		g.WritePin(pin, High)
	case High:
		g.WritePin(pin, Low)
	}
}

func (g*gpio)PullMode(pin int, pull Pull) {
	// Pull up/down/off register has offset 38 / 39, pull is 37
	pullClkReg := uint8(pin) / 32 + 38
	pullReg := 37
	shift := (uint8(pin) % 32)

	g.memlock.Lock()
	defer g.memlock.Unlock()

	switch pull {
	case PullDown, PullUp:
		g.mem[pullReg] = g.mem[pullReg] &^ 3 | uint32(pull)
	case PullOff:
		g.mem[pullReg] = g.mem[pullReg] &^ 3
	}

	// Wait for value to clock in, this is ugly, sorry :(
	time.Sleep(time.Microsecond)

	g.mem[pullClkReg] = 1 << shift

	// Wait for value to clock in
	time.Sleep(time.Microsecond)

	g.mem[pullReg] = g.mem[pullReg] &^ 3
	g.mem[pullClkReg] = 0
}
