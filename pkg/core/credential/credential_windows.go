package credential

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"agy-tools/pkg/types"
	"golang.org/x/sys/windows"
)

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")

	procCredReadW   = modadvapi32.NewProc("CredReadW")
	procCredWriteW  = modadvapi32.NewProc("CredWriteW")
	procCredDeleteW = modadvapi32.NewProc("CredDeleteW")
	procCredFree    = modadvapi32.NewProc("CredFree")
)

const (
	CRED_TYPE_GENERIC = 1
	CRED_PERSIST_LOCAL_MACHINE = 2
)

type winCREDENTIAL struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        syscall.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// Read reads the credential secret for the target from Windows Credential Manager
func Read(target string) (string, error) {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return "", err
	}

	var cred *winCREDENTIAL
	r1, _, errSys := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(CRED_TYPE_GENERIC),
		0,
		uintptr(unsafe.Pointer(&cred)),
	)

	if r1 == 0 {
		// Fallback to powershell/cmdkey if needed
		return "", fmt.Errorf("credential not found: %w", errSys)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(cred)))

	if cred == nil || cred.CredentialBlobSize == 0 || cred.CredentialBlob == nil {
		return "", nil
	}

	blobBytes := unsafe.Slice(cred.CredentialBlob, cred.CredentialBlobSize)
	return string(bytes.Clone(blobBytes)), nil
}

// Write writes the credential secret for the target to Windows Credential Manager
func Write(target, secret string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	userPtr, err := syscall.UTF16PtrFromString("antigravity-user")
	if err != nil {
		return err
	}

	secretBytes := []byte(secret)
	var blobPtr *byte
	if len(secretBytes) > 0 {
		blobPtr = &secretBytes[0]
	}

	cred := winCREDENTIAL{
		Flags:              0,
		Type:               CRED_TYPE_GENERIC,
		TargetName:         targetPtr,
		UserName:           userPtr,
		CredentialBlobSize: uint32(len(secretBytes)),
		CredentialBlob:     blobPtr,
		Persist:            CRED_PERSIST_LOCAL_MACHINE,
	}

	r1, _, errSys := procCredWriteW.Call(
		uintptr(unsafe.Pointer(&cred)),
		0,
	)

	if r1 == 0 {
		// Fallback to cmdkey
		cmd := exec.Command("cmdkey", fmt.Sprintf("/generic:%s", target), "/user:antigravity-user", fmt.Sprintf("/pass:%s", secret))
		if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return fmt.Errorf("failed to write credential via DLL (%v) and cmdkey (%v): %s", errSys, cmdErr, string(out))
		}
	}

	return nil
}

// Delete deletes the credential for the target
func Delete(target string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	r1, _, _ := procCredDeleteW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(CRED_TYPE_GENERIC),
		0,
	)

	if r1 == 0 {
		// Fallback to cmdkey
		cmd := exec.Command("cmdkey", fmt.Sprintf("/delete:%s", target))
		_ = cmd.Run()
	}

	return nil
}

// CheckReadable checks if the credential can be read
func CheckReadable() (bool, string, error) {
	secret, err := Read(types.CredentialTarget)
	if err != nil {
		if strings.Contains(err.Error(), "element not found") || strings.Contains(err.Error(), "not found") {
			return false, "", nil
		}
		return false, "", err
	}
	return true, secret, nil
}
