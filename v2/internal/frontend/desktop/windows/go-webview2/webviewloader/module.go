package webviewloader

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/jchv/go-winloader"
	"golang.org/x/sys/windows"
)

var (
	memOnce                                         sync.Once
	memModule                                       winloader.Module
	memCreate                                       winloader.Proc
	memCompareBrowserVersions                       winloader.Proc
	memGetAvailableCoreWebView2BrowserVersionString winloader.Proc
	memErr                                          error
)

const (
	// https://referencesource.microsoft.com/#system.web/Util/hresults.cs,20
	E_FILENOTFOUND = 0x80070002
)

// CompareBrowserVersions will compare the 2 given versions and return:
//     Less than zero: v1 < v2
//               zero: v1 == v2
//  Greater than zero: v1 > v2
func CompareBrowserVersions(v1 string, v2 string) (int, error) {
	_v1, err := windows.UTF16PtrFromString(v1)
	if err != nil {
		return 0, err
	}
	_v2, err := windows.UTF16PtrFromString(v2)
	if err != nil {
		return 0, err
	}

	err = loadFromMemory()
	if err != nil {
		return 0, err
	}

	var result int32
	_, _, err = memCompareBrowserVersions.Call(
		uint64(uintptr(unsafe.Pointer(_v1))),
		uint64(uintptr(unsafe.Pointer(_v2))),
		uint64(uintptr(unsafe.Pointer(&result))))

	if err != windows.ERROR_SUCCESS {
		return 0, err
	}
	return int(result), nil
}

// GetInstalledVersion returns the installed version of the webview2 runtime.
// If there is no version installed, a blank string is returned.
func GetInstalledVersion() (string, error) {
	err := loadFromMemory()
	if err != nil {
		return "", err
	}

	var result *uint16
	res, _, err := memGetAvailableCoreWebView2BrowserVersionString.Call(
		uint64(uintptr(unsafe.Pointer(nil))),
		uint64(uintptr(unsafe.Pointer(&result))))

	if res != 0 {
		if res == E_FILENOTFOUND {
			// Webview2 is not installed
			return "", nil
		}

		return "", fmt.Errorf("Unable to call GetAvailableCoreWebView2BrowserVersionString (%x): %w", res, err)
	}

	version := windows.UTF16PtrToString(result)
	windows.CoTaskMemFree(unsafe.Pointer(result))
	return version, nil
}

// CreateCoreWebView2EnvironmentWithOptions tries to load WebviewLoader2 and
// call the CreateCoreWebView2EnvironmentWithOptions routine.
func CreateCoreWebView2EnvironmentWithOptions(browserExecutableFolder, userDataFolder *uint16, environmentOptions uintptr, environmentCompletedHandle uintptr) (uintptr, error) {
	err := loadFromMemory()
	if err != nil {
		return 0, err
	}
	res, _, _ := memCreate.Call(
		uint64(uintptr(unsafe.Pointer(browserExecutableFolder))),
		uint64(uintptr(unsafe.Pointer(userDataFolder))),
		uint64(environmentOptions),
		uint64(environmentCompletedHandle),
	)
	return uintptr(res), nil
}

func loadFromMemory() error {
	var err error
	// DLL is not available natively. Try loading embedded copy.
	memOnce.Do(func() {
		memModule, memErr = winloader.LoadFromMemory(WebView2Loader)
		if memErr != nil {
			err = fmt.Errorf("Unable to load WebView2Loader.dll from memory: %w", memErr)
			return
		}
		memCreate = memModule.Proc("CreateCoreWebView2EnvironmentWithOptions")
		memCompareBrowserVersions = memModule.Proc("CompareBrowserVersions")
		memGetAvailableCoreWebView2BrowserVersionString = memModule.Proc("GetAvailableCoreWebView2BrowserVersionString")
	})
	return err
}
