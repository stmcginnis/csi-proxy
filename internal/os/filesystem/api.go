package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Implements the Filesystem OS API calls. All code here should be very simple
// pass-through to the OS APIs. Any logic around the APIs should go in
// internal/server/filesystem/server.go so that logic can be easily unit-tested
// without requiring specific OS environments.

// API is the exposed Filesystem API
type API interface {
	PathExists(path string) (bool, error)
	PathValid(path string) (bool, error)
	Mkdir(path string) error
	Rmdir(path string, force bool) error
	LinkPath(tgt string, src string) error
	IsMountPoint(path string) (bool, error)
}

type filesystemAPI struct{}

// check that filesystemAPI implements API
var _ API = &filesystemAPI{}

func New() API {
	return filesystemAPI{}
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (filesystemAPI) PathExists(path string) (bool, error) {
	return pathExists(path)
}

func pathValid(path string) (bool, error) {
	cmd := exec.Command("powershell", "/c", `Test-Path $Env:remotepath`)
	cmd.Env = append(os.Environ(), fmt.Sprintf("remotepath=%s", path))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("returned output: %s, error: %v", string(output), err)
	}

	return strings.HasPrefix(strings.ToLower(string(output)), "true"), nil
}

// PathValid determines whether all elements of a path exist
//   https://docs.microsoft.com/en-us/powershell/module/microsoft.powershell.management/test-path?view=powershell-7
// for a remote path, determines whether connection is ok
//   e.g. in a SMB server connection, if password is changed, connection will be lost, this func will return false
func (filesystemAPI) PathValid(path string) (bool, error) {
	return pathValid(path)
}

// Mkdir makes a dir with `os.MkdirAll`.
func (filesystemAPI) Mkdir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Rmdir removes a dir with `os.Remove`, if force is true then `os.RemoveAll` is used instead.
func (filesystemAPI) Rmdir(path string, force bool) error {
	if force {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// LinkPath creates newname as a symbolic link to oldname.
func (filesystemAPI) LinkPath(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

// IsMountPoint - returns true if tgt is a mount point.
// A path is considered a mount point if:
//  - directory exists and
//  - it is a soft link and
//  - the target path of the link exists.
func (filesystemAPI) IsMountPoint(tgt string) (bool, error) {
	// This code is similar to k8s.io/kubernetes/pkg/util/mount except the pathExists usage.
	// Also in a remote call environment the os error cannot be passed directly back, hence the callers
	// are expected to perform the isExists check before calling this call in CSI proxy.
	stat, err := os.Lstat(tgt)
	if err != nil {
		return false, err
	}

	// If its a link and it points to an existing file then its a mount point.
	if stat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(tgt)
		if err != nil {
			return false, fmt.Errorf("readlink error: %v", err)
		}
		exists, err := pathExists(target)
		if err != nil {
			return false, err
		}
		return exists, nil
	}

	return false, nil
}
