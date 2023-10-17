package proxmox

import (
	"fmt"
	"os"
	"path/filepath"
)

var validContent = map[string]struct{}{
	"iso":    {},
	"vztmpl": {},
}

func (s *Storage) Upload(content, file string) (*Task, error) {
	return s.upload(content, file, nil)
}

func (s *Storage) UploadWithName(content, file string, storageFilename string) (*Task, error) {
	return s.upload(content, file, &map[string]string{"filename": storageFilename})
}

func (s *Storage) UploadWithHash(content, file string, storageFilename *string, checksum, checksumAlgorithm string) (*Task, error) {
	extraArgs := map[string]string{
		"checksum":           checksum,
		"checksum-algorithm": checksumAlgorithm,
	}
	if storageFilename != nil {
		extraArgs["filename"] = *storageFilename
	}

	if storageFilename != nil {
		return s.upload(content, file, &map[string]string{"filename": *storageFilename})
	} else {
		return s.upload(content, file, nil)
	}
}

func (s *Storage) upload(content, file string, extraArgs *map[string]string) (*Task, error) {
	if _, ok := validContent[content]; !ok {
		return nil, fmt.Errorf("only iso and vztmpl allowed")
	}

	stat, err := os.Stat(file)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("file is a directory %s", file)
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var upid UPID
	data := map[string]string{"content": content}
	if extraArgs != nil {
		for k, v := range *extraArgs {
			data[k] = v
		}
	}

	if err := s.client.Upload(fmt.Sprintf("/nodes/%s/storage/%s/upload", s.Node, s.Name),
		data, f, &upid); err != nil {
		return nil, err
	}

	return NewTask(upid, s.client), nil
}

func (s *Storage) DownloadURL(content, filename, url string) (*Task, error) {
	return s.downloadURL(content, filename, url, nil)
}

func (s *Storage) DownloadURLWithHash(content, filename, url string, checksum, checksumAlgorithm string) (*Task, error) {
	return s.downloadURL(content, filename, url, &map[string]string{
		"checksum":           checksum,
		"checksum-algorithm": checksumAlgorithm,
	})
}

func (s *Storage) downloadURL(content, filename, url string, extraArgs *map[string]string) (*Task, error) {
	if _, ok := validContent[content]; !ok {
		return nil, fmt.Errorf("only iso and vztmpl allowed")
	}

	var upid UPID
	data := map[string]string{
		"content":  content,
		"filename": filename,
		"url":      url,
	}

	if extraArgs != nil {
		for k, v := range *extraArgs {
			data[k] = v
		}
	}
	err := s.client.Post(fmt.Sprintf("/nodes/%s/storage/%s/download-url", s.Node, s.Name), data, &upid)
	if err != nil {
		return nil, err
	}
	return NewTask(upid, s.client), nil
}

func (s *Storage) ISO(name string) (iso *ISO, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "iso", name), &iso)
	if err != nil {
		return nil, err
	}

	iso.client = s.client
	iso.Node = s.Node
	iso.Storage = s.Name
	if iso.VolID == "" {
		iso.VolID = fmt.Sprintf("%s:iso/%s", iso.Storage, name)
	}
	return
}

func (s *Storage) VzTmpl(name string) (vztmpl *VzTmpl, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "vztmpl", name), &vztmpl)
	if err != nil {
		return nil, err
	}

	vztmpl.client = s.client
	vztmpl.Node = s.Node
	vztmpl.Storage = s.Name
	if vztmpl.VolID == "" {
		vztmpl.VolID = fmt.Sprintf("%s:vztmpl/%s", vztmpl.Storage, name)
	}
	return
}

func (s *Storage) Backup(name string) (backup *Backup, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "backup", name), &backup)
	if err != nil {
		return nil, err
	}

	backup.client = s.client
	backup.Node = s.Node
	backup.Storage = s.Name
	return
}

func (v *VzTmpl) Delete() (*Task, error) {
	return deleteVolume(v.client, v.Node, v.Storage, v.VolID, v.Path, "vztmpl")
}

func (b *Backup) Delete() (*Task, error) {
	return deleteVolume(b.client, b.Node, b.Storage, b.VolID, b.Path, "backup")
}

func (i *ISO) Delete() (*Task, error) {
	return deleteVolume(i.client, i.Node, i.Storage, i.VolID, i.Path, "iso")
}

func deleteVolume(c *Client, n, s, v, p, t string) (*Task, error) {
	var upid UPID
	if v == "" && p == "" {
		return nil, fmt.Errorf("volid or path required for a delete")
	}

	if v == "" {
		// volid not returned in the volume endpoints, need to generate
		v = fmt.Sprintf("%s:%s/%s", s, t, filepath.Base(p))
	}

	err := c.Delete(fmt.Sprintf("/nodes/%s/storage/%s/content/%s", n, s, v), &upid)
	return NewTask(upid, c), err
}
