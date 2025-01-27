package internal

import (
	"archive/zip"
	"io"

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

func ZipClone(reader *zip.ReadCloser, writer *zip.Writer) error {
	for _, zipFile := range reader.File {
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

func ZipSetText(z *zip.Writer, filename string, data string) error {
	w, err := z.Create(filename)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(data))
	return err
}
