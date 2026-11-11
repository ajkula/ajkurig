package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"time"
)

// ===== Types =====

type ArchiveEntry struct {
	Name       string
	Mode       int64
	SourcePath string
	Content    []byte
}

// ===== Public API =====

func writeTarGz(archivePath string, entries []ArchiveEntry) (err error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer joinCloseError(&err, file)
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		if err := streamTarEntry(archive, entry); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	return compressed.Close()
}

func writeZip(archivePath string, entries []ArchiveEntry) (err error) {
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer joinCloseError(&err, file)
	archive := zip.NewWriter(file)
	for _, entry := range entries {
		if err := streamZipEntry(archive, entry); err != nil {
			return err
		}
	}
	return archive.Close()
}

// ===== Internal =====

func joinCloseError(err *error, closer io.Closer) {
	*err = errors.Join(*err, closer.Close())
}

func streamTarEntry(archive *tar.Writer, entry ArchiveEntry) (err error) {
	reader, size, err := openEntry(entry)
	if err != nil {
		return err
	}
	defer joinCloseError(&err, reader)
	header := &tar.Header{Name: entry.Name, Mode: entry.Mode, Size: size, ModTime: time.Now(), Typeflag: tar.TypeReg}
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(archive, reader)
	return err
}

func streamZipEntry(archive *zip.Writer, entry ArchiveEntry) (err error) {
	reader, _, err := openEntry(entry)
	if err != nil {
		return err
	}
	defer joinCloseError(&err, reader)
	header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate, Modified: time.Now()}
	header.SetMode(os.FileMode(entry.Mode))
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, reader)
	return err
}

func openEntry(entry ArchiveEntry) (io.ReadCloser, int64, error) {
	if entry.SourcePath == "" {
		return io.NopCloser(bytesReader(entry.Content)), int64(len(entry.Content)), nil
	}
	file, err := os.Open(entry.SourcePath)
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}
