package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ===== Constants =====

const (
	fatMagic          = 0xCAFEBABE
	fatAlignmentPower = 14
	machHeaderSize    = 12
	fatHeaderSize     = 8
	fatArchSize       = 20
)

var macArchitectures = []string{"arm64", "amd64"}

const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>%[1]s</string>
	<key>CFBundleDisplayName</key><string>%[1]s</string>
	<key>CFBundleExecutable</key><string>%[1]s</string>
	<key>CFBundleIdentifier</key><string>%[2]s</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>%[3]s</string>
	<key>CFBundleVersion</key><string>%[3]s</string>
	<key>LSMinimumSystemVersion</key><string>12.0</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>LSApplicationCategoryType</key><string>%[4]s</string>
</dict>
</plist>
`

// ===== Internal =====

func packageMacOS(release *Release, archivePath string) error {
	slicePaths := make([]string, 0, len(macArchitectures))
	for _, goarch := range macArchitectures {
		binary := filepath.Join(release.WorkDir, "darwin-"+goarch)
		if err := compile(release, "darwin", goarch, false, stripFlags, binary); err != nil {
			return err
		}
		slicePaths = append(slicePaths, binary)
	}
	universal := filepath.Join(release.WorkDir, "darwin-universal")
	if err := mergeFatBinary(universal, slicePaths); err != nil {
		return err
	}
	config := release.Config
	bundle := config.Name + ".app/Contents/"
	return writeTarGz(archivePath, []ArchiveEntry{
		{Name: bundle + "MacOS/" + config.Name, Mode: 0o755, SourcePath: universal},
		{Name: bundle + "Info.plist", Mode: 0o644, Content: fmt.Appendf(nil, infoPlist, config.Name, config.BundleID, release.Version, config.Category)},
	})
}

func mergeFatBinary(output string, inputs []string) error {
	alignment := int64(1) << fatAlignmentPower
	header := new(bytes.Buffer)
	writeBigEndian(header, uint32(fatMagic), uint32(len(inputs)))
	offset := alignUp(int64(fatHeaderSize+fatArchSize*len(inputs)), alignment)
	layout := make([]int64, len(inputs))
	for index, input := range inputs {
		cpuType, cpuSubtype, size, err := readMachHeader(input)
		if err != nil {
			return err
		}
		layout[index] = offset
		writeBigEndian(header, cpuType, cpuSubtype, uint32(offset), uint32(size), uint32(fatAlignmentPower))
		offset = alignUp(offset+size, alignment)
	}
	return writeFatFile(output, header.Bytes(), inputs, layout)
}

func writeFatFile(output string, header []byte, inputs []string, layout []int64) error {
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(header); err != nil {
		return err
	}
	for index, input := range inputs {
		if err := copySliceAt(file, input, layout[index]); err != nil {
			return err
		}
	}
	return nil
}

func copySliceAt(file *os.File, input string, offset int64) error {
	source, err := os.Open(input)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(file, source)
	return err
}

func readMachHeader(path string) (uint32, uint32, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()
	header := make([]byte, machHeaderSize)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, 0, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, 0, 0, err
	}
	return binary.LittleEndian.Uint32(header[4:]), binary.LittleEndian.Uint32(header[8:]), info.Size(), nil
}

func writeBigEndian(buffer *bytes.Buffer, values ...uint32) {
	for _, value := range values {
		_ = binary.Write(buffer, binary.BigEndian, value)
	}
}

func alignUp(value, alignment int64) int64 {
	return (value + alignment - 1) / alignment * alignment
}

func bytesReader(content []byte) io.Reader {
	return bytes.NewReader(content)
}
