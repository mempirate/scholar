package store

import (
	"io"
	"os"
	"path/filepath"
)

type LocalStore interface {
	// List returns a list of all files in the store.
	List() ([]string, error)

	Contains(name string) (bool, error)

	Store(name string, content io.Reader) error

	// Get returns a reader for the file with the given name. The caller is responsible for closing the reader!
	Get(name string) (io.ReadCloser, error)
}

type FileStore struct {
	dataDir string
}

func NewFileStore(dataDir string) *FileStore {
	return &FileStore{
		dataDir: dataDir,
	}
}

func (fs *FileStore) List() ([]string, error) {
	entries, err := os.ReadDir(fs.dataDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func (fs *FileStore) Contains(name string) (bool, error) {
	_, err := os.Stat(filepath.Join(fs.dataDir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (fs *FileStore) Store(name string, content io.Reader) error {
	filePath := filepath.Join(fs.dataDir, name)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	return err
}

func (fs *FileStore) Get(name string) (io.ReadCloser, error) {
	filePath := filepath.Join(fs.dataDir, name)
	return os.Open(filePath)
}
