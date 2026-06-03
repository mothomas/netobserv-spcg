package pcap

import "errors"

// S3CaptureConfig holds tenant-provided object storage settings for a capture session.
// Credentials are held in memory only for the session lifetime and are never written to logs.
type S3CaptureConfig struct {
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint,omitempty"`
	Region          string `json:"region,omitempty"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix,omitempty"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
	ForcePathStyle  bool   `json:"force_path_style,omitempty"`
	ProxyURL        string `json:"proxy_url,omitempty"`
}

// S3ExportInfo is returned to the UI after capture or on download request.
type S3ExportInfo struct {
	Enabled    bool   `json:"enabled"`
	Bucket     string `json:"bucket,omitempty"`
	ObjectKey  string `json:"object_key,omitempty"`
	ObjectURL  string `json:"object_url,omitempty"`
	Bytes      uint64 `json:"bytes,omitempty"`
	UploadDone bool   `json:"upload_done"`
}

func (c S3CaptureConfig) ValidForCapture() error {
	if !c.Enabled {
		return nil
	}
	if c.Bucket == "" {
		return errors.New("s3 bucket is required")
	}
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		return errors.New("s3 access key and secret are required")
	}
	return nil
}
