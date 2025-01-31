package internal

import (
	"archive/zip"
	"io"
	"io/fs"
	"slices"

	_ "golang.org/x/text/encoding/charmap"
)

func ZipGetText(z *zip.ReadCloser, filename string) (string, error) {
	rc, err := z.Open(filename)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func ZipClone(reader *zip.ReadCloser, writer *zip.Writer, except []string) error {
	for _, zipFile := range reader.File {
		if slices.Contains(except, zipFile.Name) {
			continue
		}
		rc, err := zipFile.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		w, err := writer.Create(zipFile.Name)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func ZipSet(z *zip.Writer, filename string, data []byte) error {
	w, err := z.Create(filename)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

type ZipArchive struct {
	reader *zip.ReadCloser
	writer *zip.Writer
	files  map[string][]byte
}

func NewZipArchive(name string, w io.Writer) (*ZipArchive, error) {
	reader, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	writer := zip.NewWriter(w)
	return &ZipArchive{
		reader: reader,
		writer: writer,
		files:  make(map[string][]byte),
	}, nil
}

func (za *ZipArchive) SetFile(name string, data []byte) {
	za.files[name] = data
}

func (za *ZipArchive) GetFile(name string) ([]byte, error) {
	if data, ok := za.files[name]; ok {
		return data, nil
	}

	rc, err := za.reader.Open(name)
	if err != nil {
		if _, ok := err.(*fs.PathError); ok {
			file, ok := za.files[name]
			if ok {
				return file, nil
			}
		}
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (za *ZipArchive) Close() error {
	names := make([]string, len(za.files))
	i := 0
	for name, data := range za.files {
		err := ZipSet(za.writer, name, data)
		if err != nil {
			return err
		}
		names[i] = name
		i++
	}

	for _, file := range za.reader.File {
		if slices.Contains(names, file.Name) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		w, err := za.writer.Create(file.Name)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
	}

	return za.writer.Close()
}
