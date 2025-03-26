package proxmox

import (
	"context"
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
	}

	return s.upload(content, file, nil)
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
	defer func() { _ = f.Close() }()

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

func (s *Storage) DownloadURL(ctx context.Context, content, filename, url string) (*Task, error) {
	return s.downloadURL(ctx, content, filename, url, nil)
}

func (s *Storage) DownloadURLWithHash(ctx context.Context, content, filename, url string, checksum, checksumAlgorithm string) (*Task, error) {
	return s.downloadURL(ctx, content, filename, url, &map[string]string{
		"checksum":           checksum,
		"checksum-algorithm": checksumAlgorithm,
	})
}

func (s *Storage) downloadURL(ctx context.Context, content, filename, url string, extraArgs *map[string]string) (*Task, error) {
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
	err := s.client.Post(ctx, fmt.Sprintf("/nodes/%s/storage/%s/download-url", s.Node, s.Name), data, &upid)
	if err != nil {
		return nil, err
	}
	return NewTask(upid, s.client), nil
}

func (s *Storage) ISO(ctx context.Context, name string) (iso *ISO, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "iso", name), &iso)
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

func (s *Storage) VzTmpl(ctx context.Context, name string) (vztmpl *VzTmpl, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "vztmpl", name), &vztmpl)
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

func (s *Storage) Backup(ctx context.Context, name string) (backup *Backup, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "backup", name), &backup)
	if err != nil {
		return nil, err
	}

	backup.client = s.client
	backup.Node = s.Node
	backup.Storage = s.Name
	return
}

func (s *Storage) Snippet(ctx context.Context, name string) (snippet *Snippet, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "snippets", name), &snippet)
	if err != nil {
		return nil, err
	}

	snippet.client = s.client
	snippet.Node = s.Node
	snippet.Storage = s.Name
	return
}

func (s *Storage) Image(ctx context.Context, name string) (image *Image, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s", s.Node, s.Name, s.Name, name), &image)
	if err != nil {
		return nil, err
	}

	image.client = s.client
	image.Node = s.Node
	image.Storage = s.Name
	return
}

func (s *Storage) Contents(ctx context.Context) (contents *Contents, err error) {
	err = s.client.Get(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content", s.Node, s.Name), &contents)
	if err != nil {
		return nil, err
	}

	for _, content := range *contents {
		content.client = s.client
		content.Node = s.Node
		content.Storage = s.Name
	}
	return
}

func (s *Storage) Delete(ctx context.Context, volID string) (*Task, error) {
	return deleteVolume(ctx, s.client, s.Node, s.Name, volID, "", "")
}

func (v *VzTmpl) Delete(ctx context.Context) (*Task, error) {
	return deleteVolume(ctx, v.client, v.Node, v.Storage, v.VolID, v.Path, "vztmpl")
}

func (b *Backup) Delete(ctx context.Context) (*Task, error) {
	return deleteVolume(ctx, b.client, b.Node, b.Storage, b.VolID, b.Path, "backup")
}

func (i *ISO) Delete(ctx context.Context) (*Task, error) {
	return deleteVolume(ctx, i.client, i.Node, i.Storage, i.VolID, i.Path, "iso")
}

func deleteVolume(ctx context.Context, c *Client, n, s, v, p, t string) (*Task, error) {
	var upid UPID
	if v == "" && p == "" {
		return nil, fmt.Errorf("volid or path required for a delete")
	}

	if v == "" {
		// volid not returned in the volume endpoints, need to generate
		v = fmt.Sprintf("%s:%s/%s", s, t, filepath.Base(p))
	}

	err := c.Delete(ctx, fmt.Sprintf("/nodes/%s/storage/%s/content/%s", n, s, v), &upid)
	return NewTask(upid, c), err
}

func (c *Client) Storages(ctx context.Context) (*CStorages, error) {

	storage := CStorages{}
	if err := c.Get(ctx, "/storage", &storage); err != nil {
		return nil, err
	}

	for _, s := range storage {
		s.client = s.client
	}

	return &storage, nil
}

func (c *Client) Storage(ctx context.Context, name string) (*CStorage, error) {

	storage := CStorage{}
	if err := c.Get(ctx, fmt.Sprintf("/storage/%s", name), &storage); err != nil {
		return nil, err
	}
	storage.client = c

	return &storage, nil
}

func (c *Client) NewStorage(ctx context.Context, name string, storageType string, options map[string]string) (*CStorage, error) {
	data := make(map[string]string)
	for k, v := range options {
		data[k] = v
	}
	data["storage"] = name
	data["type"] = storageType

	var storage CStorage
	if err := c.Post(ctx, "/storage", data, &storage); err != nil {
		return nil, err
	}
	storage.client = c

	return &storage, nil
}

func (s *CStorage) Delete(ctx context.Context) error {
	return s.client.Delete(ctx, fmt.Sprintf("/storage/%s", s.Name), nil)
}
