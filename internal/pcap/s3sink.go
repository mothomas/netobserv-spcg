package pcap

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

const s3PartSize = 5 * 1024 * 1024

// TestS3Connection verifies bucket access using a short-lived probe object.
func TestS3Connection(ctx context.Context, cfg S3CaptureConfig) error {
	if err := cfg.ValidForCapture(); err != nil {
		return err
	}
	client, err := newS3Client(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)})
	if err != nil {
		return fmt.Errorf("s3 head bucket: %w", err)
	}
	key := path.Join(strings.Trim(cfg.Prefix, "/"), ".spcg-probe", uuid.NewString())
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("spcg-ok"),
	})
	if err != nil {
		return fmt.Errorf("s3 put probe: %w", err)
	}
	_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	return nil
}

// S3Sink streams PCAP-NG blocks directly to S3 via multipart upload.
type S3Sink struct {
	cfg       S3CaptureConfig
	client    *s3.Client
	bucket    string
	key       string
	uploadID  string
	parts     []completedPart
	partBuf   bytes.Buffer
	partNum   int32
	bytes     uint64
	headerOK  bool
	mu        sync.Mutex
	closed    bool
}

type completedPart struct {
	etag string
	num  int32
}

func NewS3Sink(ctx context.Context, cfg S3CaptureConfig, captureID string) (*S3Sink, error) {
	if err := cfg.ValidForCapture(); err != nil {
		return nil, err
	}
	client, err := newS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	key := path.Join(prefix, captureID, "merged.pcapng")
	out, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String("application/vnd.tcpdump.pcap"),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 create multipart upload: %w", err)
	}
	s := &S3Sink{
		cfg: cfg, client: client, bucket: cfg.Bucket,
		key: key, uploadID: aws.ToString(out.UploadId),
	}
	if err := s.writeHeaderLocked(); err != nil {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(s.bucket), Key: aws.String(s.key), UploadId: aws.String(s.uploadID),
		})
		return nil, err
	}
	return s, nil
}

func (s *S3Sink) ObjectKey() string { return s.key }

func (s *S3Sink) BytesUploaded() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytes
}

func (s *S3Sink) writeHeaderLocked() error {
	var hdr bytes.Buffer
	writeSHB(&hdr)
	writeIDB(&hdr)
	s.partBuf.Write(hdr.Bytes())
	s.bytes += uint64(hdr.Len())
	s.headerOK = true
	return s.flushPartLocked(context.Background(), false)
}

func (s *S3Sink) WriteFrame(data []byte, at time.Time) error {
	if len(data) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("s3 sink closed")
	}
	if !s.headerOK {
		if err := s.writeHeaderLocked(); err != nil {
			return err
		}
	}
	var epb bytes.Buffer
	if at.IsZero() {
		at = time.Now().UTC()
	}
	writeEPB(&epb, data, at)
	s.partBuf.Write(epb.Bytes())
	s.bytes += uint64(len(data))
	return s.flushPartLocked(context.Background(), false)
}

func (s *S3Sink) flushPartLocked(ctx context.Context, force bool) error {
	if s.partBuf.Len() < s3PartSize && !force {
		return nil
	}
	if s.partBuf.Len() == 0 {
		return nil
	}
	s.partNum++
	body := bytes.NewReader(s.partBuf.Bytes())
	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key),
		UploadId: aws.String(s.uploadID), PartNumber: aws.Int32(s.partNum), Body: body,
	})
	if err != nil {
		return fmt.Errorf("s3 upload part: %w", err)
	}
	s.parts = append(s.parts, completedPart{etag: aws.ToString(out.ETag), num: s.partNum})
	s.partBuf.Reset()
	return nil
}

func (s *S3Sink) Close(ctx context.Context) (*S3ExportInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.infoLocked(true), nil
	}
	if err := s.flushPartLocked(ctx, true); err != nil {
		_, _ = s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(s.bucket), Key: aws.String(s.key), UploadId: aws.String(s.uploadID),
		})
		return nil, err
	}
	completed := make([]types.CompletedPart, 0, len(s.parts))
	for _, p := range s.parts {
		completed = append(completed, types.CompletedPart{
			ETag: aws.String(p.etag), PartNumber: aws.Int32(p.num),
		})
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(s.bucket), Key: aws.String(s.key), UploadId: aws.String(s.uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return nil, fmt.Errorf("s3 complete multipart: %w", err)
	}
	s.closed = true
	info := s.infoLocked(true)
	url, err := presignGet(ctx, s.client, s.bucket, s.key)
	if err == nil {
		info.ObjectURL = url
	}
	return info, nil
}

func (s *S3Sink) infoLocked(done bool) *S3ExportInfo {
	return &S3ExportInfo{
		Enabled: true, Bucket: s.bucket, ObjectKey: s.key,
		Bytes: s.bytes, UploadDone: done,
	}
}

func presignGet(ctx context.Context, client *s3.Client, bucket, key string) (string, error) {
	ps := s3.NewPresignClient(client)
	out, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
	}, s3.WithPresignExpires(7*24*time.Hour))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func newS3Client(ctx context.Context, cfg S3CaptureConfig) (*s3.Client, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)
	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithCredentialsProvider(creds),
	}
	if cfg.ProxyURL != "" {
		u, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("s3 proxy url: %w", err)
		}
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.Proxy = http.ProxyURL(u)
		loadOpts = append(loadOpts, config.WithHTTPClient(&http.Client{Transport: tr}))
	}
	awscfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awscfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		if cfg.ForcePathStyle || cfg.Endpoint != "" {
			o.UsePathStyle = true
		}
	})
	return client, nil
}

func PresignS3Object(ctx context.Context, cfg S3CaptureConfig, objectKey string) (string, error) {
	client, err := newS3Client(ctx, cfg)
	if err != nil {
		return "", err
	}
	return presignGet(ctx, client, cfg.Bucket, objectKey)
}
