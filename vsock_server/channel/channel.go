// go:build amd64 && windows
package channel

import (
	"errors"
	"fmt"
	"log"
	"math"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ErrInvalidHandle = errors.New("vmci: invalid handle")
)

const (
	svmZeroSize              = 4
	IOCTL_GET_AF      uint32 = 0x0801300C
	socketsDevicePath        = "\\??\\Viosock"
	VMAddrCIDAny             = uint32(math.MaxUint32)
)

type (
	controlCode uint32
	saFamily    uint16
)

type sockAddrVM struct {
	family    saFamily
	reserved1 uint16
	port      uint32
	cid       uint32
	zero      [svmZeroSize]byte
}

func (sa *sockAddrVM) sockaddr() (unsafe.Pointer, int32) {
	size := unsafe.Sizeof(*sa)
	return unsafe.Pointer(sa), int32(size)
}

func openSocketDevice() (windows.Handle, error) {
	socketsDevicePathW, err := windows.UTF16PtrFromString(socketsDevicePath)
	hDevice, err := windows.CreateFile(socketsDevicePathW, windows.GENERIC_READ, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return 0, fmt.Errorf("Viosock: failed to open Viosock device: %w", err)
	}

	if hDevice == windows.InvalidHandle {
		return 0, ErrInvalidHandle
	}

	return hDevice, nil
}

func deviceIOControl() (uint32, error) {
	hDevice, err := openSocketDevice()
	if err != nil {
		return 0, err
	}

	defer func() {
		_ = windows.CloseHandle(hDevice)
	}()

	// overflow trick is used very often in original source code as:
	// 	unsigned int val = (unsigned int)-1;
	val := uint32(math.MaxUint32)

	// dirty trick to pass val as *byte and mitigate type signature mismatch
	valPtr := (*byte)(unsafe.Pointer(&val))
	valSize := uint32(unsafe.Sizeof(val))

	var ioReturn uint32
	err = windows.DeviceIoControl(hDevice, IOCTL_GET_AF, valPtr, valSize, valPtr, valSize, &ioReturn, nil)
	if err != nil {
		return 0, err
	}

	return val, nil
}

func resultAsInt(val uint32, err error) (int, error) {
	i32 := (int32)(val)
	return int(i32), err
}

func GetAFValue() (int, error) {
	return resultAsInt(deviceIOControl())
}

func newSockAddr(family saFamily, port, cid uint32) *sockAddrVM {
	return &sockAddrVM{
		family: family,
		cid:    cid,
		port:   port,
	}
}

//go:linkname syscallConnect syscall.connect
func syscallConnect(fd int, name unsafe.Pointer, namelen int32) (err error)

type channel struct {
	port         uint32
	cidr         uint32
	vsock_handle syscall.Handle
}

func New(port uint32, cidr uint32) channel {
	return channel{port, cidr, 0}
}

func (c *channel) Init() {
	af, err := GetAFValue()
	if err != nil {
		panic(err)
	}

	c.vsock_handle, err = syscall.Socket(int(af), 1, 0)
	if err != nil {
		panic(err)
	}

	addr := newSockAddr(saFamily(af), c.port, c.cidr)
	addrPtr, ptrSize := addr.sockaddr()
	err = syscallConnect(int(c.vsock_handle), addrPtr, ptrSize)
	if err != nil {
		panic(err)
	}

	log.Printf("VC channel connected")
}

func (c *channel) Read(buf []byte) (int, error) {
	length := len(buf)

	wsaBuffer := syscall.WSABuf{Len: uint32(length), Buf: &buf[0]}

	n := uint32(0)
	flags := uint32(0)

	err := syscall.WSARecv(c.vsock_handle, &wsaBuffer, 1, &n, &flags, nil, nil)
	return int(n), err
}

func (c *channel) Write(buf []byte) (int, error) {
	length := len(buf)

	wsaBuffer := syscall.WSABuf{Len: uint32(length), Buf: &buf[0]}

	n := uint32(0)
	flags := uint32(0)

	err := syscall.WSASend(c.vsock_handle, &wsaBuffer, 1, &n, flags, nil, nil)
	return int(n), err
}

func (c *channel) Close() error {
	return syscall.Close(c.vsock_handle)
}
