// Code generated by go-bindata.
// sources:
// sample-obcrawler.conf
// DO NOT EDIT!

package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _sampleObcrawlerConf = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x8c\x54\x4d\x6f\xe3\x36\x10\xbd\xeb\x57\xcc\x61\x17\x68\x01\x41\x8a\x93\x45\x8b\xae\xe1\x83\xd3\xe4\x10\x24\xbb\x36\xe2\x7c\x2c\xf6\x36\x12\xc7\x16\xbb\x14\x87\xe1\x50\x56\xdc\xa2\xf9\xed\xc5\x48\x76\x3e\x76\x5b\xa0\xf0\xc1\x22\xf9\xe6\xf1\xcd\x9b\x19\x4e\xe1\xa6\x21\x30\x36\x52\x9d\x38\xee\x20\x31\x48\xe2\x48\x60\x30\x21\x48\x57\x37\x80\x02\xa9\x21\xe0\x40\xbe\xc2\x3f\x11\xe3\x70\x56\xa1\x50\x0e\x36\xac\x05\x5a\x4a\xa8\x5b\x39\xa0\x37\xd9\x14\x42\x57\x39\x5b\x0f\xa8\x22\xdb\x5f\x40\x6b\xec\x5c\x02\x2b\xf0\x54\x16\xaf\xa8\xd8\xc3\x72\xb1\xba\xf8\x02\x8b\x15\x49\x0e\xef\xae\x16\xbf\xcf\xaf\xe6\xcb\xe5\xd9\xfc\x66\x5e\x2e\x02\xf9\xd3\x67\xdc\xbd\xf5\x86\x7b\xc9\xb3\x29\x3c\x95\x57\xb6\x8a\x18\x77\xe5\x3c\x04\x67\x6b\x4c\x96\x3d\xac\xba\x10\x38\xa6\xef\xc2\x3e\x61\x0d\x8b\xd5\xa0\x0d\xde\x35\xdc\x52\xf9\xe6\xfa\x6c\x0a\x4b\x87\xfe\xb7\x02\xe0\xdc\x6f\x6d\x64\xdf\x92\x4f\xb0\xc5\x68\xb1\x72\x24\x80\x91\x80\x1e\x03\x7a\x43\x06\x84\xd5\x8b\x1d\xb4\xb8\x83\x8a\xa0\x13\x32\x05\xc0\xe7\xc5\xcd\xf9\xc7\x83\xbe\x6c\x0a\xf4\x9f\x44\x69\x17\x6c\x8d\xce\xed\xe0\xfd\xdd\xfc\xfa\x62\x7e\x7a\x75\xfe\x3e\x87\xaa\x4b\x7b\xda\x4e\x92\xf2\x62\x5d\x93\x08\x19\xe8\x6d\x6a\xb2\x29\xbc\x3b\x80\xa1\xa1\x48\x05\xc0\xdc\x09\xe7\xf0\xa4\x7e\x3e\x6b\x4b\xfc\xd6\xbe\x57\x9e\x69\x19\xb4\x1c\xc6\xc6\xd9\x1b\xff\xb3\x6c\x0a\xb7\x42\x90\x48\x92\xa7\xa4\xb8\xfd\xe7\x6c\x32\x9c\x79\xbb\xa5\x28\xe8\x60\xe9\xba\xcd\xe0\xe1\xd2\xe1\x0e\x7e\xba\x5d\xfa\xe5\xcf\x80\x5d\xe2\x16\xd3\x3e\x25\xa5\x1d\x7b\xc5\x59\x49\xe4\x41\xab\x01\x5c\x25\xb4\x5e\x6d\xd1\x13\x7a\x4c\x14\x3d\x3a\xb8\x58\x02\x1a\x13\x49\x04\xd6\x91\x5b\x90\xb1\x78\x64\xc0\xd0\xd6\xd6\x24\x05\xdc\x34\x56\x80\xc3\x50\x5b\x63\x65\x74\xd1\x0e\x22\x3d\x77\xc1\x87\x51\xe3\x29\x73\x92\x84\xe1\xc0\xb7\xb7\x5a\x6b\xa3\x9e\xfc\xc1\xd6\x0f\x57\x7b\x4a\x3d\xc7\x6f\x05\x2c\x3c\x48\xc2\x98\xc6\x5d\x36\x04\xbd\x75\x0e\x5a\xfc\x46\xd9\x14\xb8\x4b\x1b\xb6\x7e\x03\x35\x7b\x4f\xb5\xde\x2e\xca\xa3\xe0\x6a\xb8\x2a\x62\x80\x40\x14\x65\xf0\xa3\x53\xfb\x1a\x6a\x15\x63\xac\xd4\xbc\xa5\x08\x9c\x1a\x8a\x3a\x0a\x03\xec\x3b\x01\xd9\xf4\x85\x48\x35\xcf\x4a\x1b\x3e\x94\x8f\xc5\xf0\x2b\x53\x1d\xca\x0f\x47\x47\x93\x32\x1c\x87\x72\x72\x7c\x76\x72\xc9\x7c\x7f\x7a\xbe\xaa\x8f\xd3\xca\x6f\xef\xae\xa9\xbd\x14\x39\xfb\x64\x2f\xaf\xbe\xd2\xe5\xfa\xf6\xba\xe9\xbf\x60\xff\xf5\x1e\x2d\x3f\xc8\xf2\x64\x3b\xe9\xb3\xfd\xcc\xf9\xae\xad\x54\xca\x1a\x22\x49\x60\xaf\xc6\x24\x86\x1e\x6d\x82\x35\x47\xe8\x1b\xf2\x9a\xb4\xe6\x7a\xb1\xfc\xbc\x82\x87\x8e\xa2\x7d\x36\xde\x0a\x20\xa4\x88\x86\x78\xbd\x56\xc9\x94\x7a\xa2\x31\x13\xac\xeb\x2e\x62\xbd\x53\x72\x5d\x6b\xe4\x6e\x70\x43\x57\x12\x88\x8c\x66\x69\x83\x97\x87\x8e\x63\xd7\xce\x3e\xa8\xaa\x79\x08\xe4\x0d\x20\xd4\xdc\x0e\xc3\xb1\xb7\xb5\x13\x8a\x80\x1b\xdd\xd9\x5b\xf5\xea\x09\x79\x79\x9c\x94\xb2\xc3\x7d\xec\x6c\xff\xaf\xbc\x67\x54\x75\x1b\x70\xbc\xd9\x68\x2e\x8e\xb6\xe4\x14\x7b\x87\xce\x9a\x71\x39\xb6\xc4\x5f\x46\x81\x39\x58\xbf\xe6\x1c\x3c\x27\x5b\x53\x0e\x3d\x46\x6f\xfd\x26\x07\x8a\x91\x63\x0e\x75\xb4\x43\x47\xff\x9d\x4d\x95\x73\x88\x9f\x69\xc8\xc1\xd8\x7f\x79\x2d\x1d\x6f\x60\x6d\x1d\xc9\x18\xf3\xc3\x9c\x95\x8e\x37\x32\xc6\x8f\xd6\x6a\x92\xa6\xd2\xf7\x80\x0a\xb8\x48\x50\xa3\x07\xb2\xda\x35\x3a\xff\xf2\xe0\x6c\xa2\x93\x1c\xda\x9d\x3c\xb8\x1c\x38\x42\x60\x49\x1b\x6d\xef\x61\x96\x2b\x63\xd1\x51\x9d\x66\x03\xe0\x20\xac\x61\x49\x07\x72\xfd\xfe\x38\x0c\xa0\xd6\x5a\x77\x06\x28\xec\x17\xcf\x74\x20\x14\xb7\x14\x47\x56\x0d\x9a\x4d\x8e\x7f\x2d\x8e\x8a\xa3\x62\xf2\xf1\xe4\xe4\xe8\x97\x03\xb7\xd6\xc8\x63\x4b\x3f\xd2\xbd\x50\x99\x6a\xa4\x51\xec\xec\x10\x70\x20\x08\x28\xd2\x73\x34\xff\x87\x40\xb1\xb3\x43\xc0\x3f\x01\x00\x00\xff\xff\xd5\x34\xf6\x21\xa3\x06\x00\x00")

func sampleObcrawlerConfBytes() ([]byte, error) {
	return bindataRead(
		_sampleObcrawlerConf,
		"sample-obcrawler.conf",
	)
}

func sampleObcrawlerConf() (*asset, error) {
	bytes, err := sampleObcrawlerConfBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "sample-obcrawler.conf", size: 1699, mode: os.FileMode(420), modTime: time.Unix(1582424557, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"sample-obcrawler.conf": sampleObcrawlerConf,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"sample-obcrawler.conf": &bintree{sampleObcrawlerConf, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
