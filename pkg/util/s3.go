package util

import (
	"context"
	"strconv"

	"emperror.dev/errors"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	S3AccessKeyName = "ACCESS_KEY"
	S3SecretKeyName = "SECRET_KEY"
)

type S3Config struct {
	BucketName      string
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type S3ConfigExtractor struct {
	client    *client.Client
	Namespace string
	S3Spec    *s3v1alpha1.S3BucketSpec
}

func NewS3ConfigExtractor(client *client.Client, s3Spec *s3v1alpha1.S3BucketSpec, namespace string) *S3ConfigExtractor {
	return &S3ConfigExtractor{
		client:    client,
		Namespace: namespace,
		S3Spec:    s3Spec,
	}
}

func (s *S3ConfigExtractor) GetS3Config(ctx context.Context) (s3info *S3Config, err error) {
	if s.S3Spec == nil {
		return
	}
	connRef := s.S3Spec.Connection

	if connRef == nil {
		err = errors.New("S3 connection reference is nil")
		return
	} else {
		if connRefName := connRef.Reference; connRefName != "" {
			s3info, err = s.GetS3ConfigFromConnectionReferenceName(ctx, connRefName)
		} else {
			s3info, err = s.GetS3ConfigFromConnectionInline(ctx, s.S3Spec.Connection)
		}
	}
	return
}

// get s3 connection info from inline
func (s *S3ConfigExtractor) GetS3ConfigFromConnectionInline(ctx context.Context, connRef *s3v1alpha1.S3BucketConnectionSpec) (s3Info *S3Config, err error) {
	if connInline := connRef.Inline; connInline == nil {
		err = errors.NewWithDetails("S3 connection inline and reference name cannot be both nil", "S3Spec", s.S3Spec, "namespace", s.Namespace)
		return
	} else {
		s3Info, err = s.GetS3ConfigFromConnectionSpec(ctx, connInline)
	}
	return
}

// create s3 info with s3v1alpha1.S3ConnectionSpec
func (s *S3ConfigExtractor) GetS3ConfigFromConnectionSpec(ctx context.Context, connSpec *s3v1alpha1.S3ConnectionSpec) (s3Info *S3Config, err error) {
	var accessKey, secretKey string
	accessKey, secretKey, err = s.GetS3SecretData(ctx, connSpec.Credentials)
	if err != nil {
		err = errors.WrapWithDetails(err, "failed to get s3 secret data", "credentials", connSpec.Credentials, "namespace", s.Namespace)
		return
	}
	s3Info = &S3Config{
		BucketName:      s.S3Spec.BucketName,
		Endpoint:        s.GetS3ConnectionInlineEndpoint(),
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		ForcePathStyle:  connSpec.PathStyle,
	}
	return
}

// get s3 connection info from reference bane
func (s *S3ConfigExtractor) GetS3ConfigFromConnectionReferenceName(ctx context.Context, connRefName string) (s3Info *S3Config, err error) {
	if connRefName == "" {
		err = errors.NewWithDetails("S3 connection reference name cannot be empty", "S3Spec", s.S3Spec, "namespace", s.Namespace)
		return
	}

	s3Conn := &s3v1alpha1.S3Connection{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      connRefName,
		},
	}

	err = s.client.GetWithObject(ctx, s3Conn)
	if err != nil {
		err = errors.WrapWithDetails(err, "failed to get s3 connection with connection reference name", "ref name", connRefName, "namespace", s.Namespace)
		return
	}

	s3Info, err = s.GetS3ConfigFromConnectionSpec(ctx, &s3Conn.Spec)
	return
}

// endpoint
func (s *S3ConfigExtractor) GetS3ConnectionInlineEndpoint() string {
	port := s.S3Spec.Connection.Inline.Port
	if port == 0 {
		return s.S3Spec.Connection.Inline.Host
	}
	return s.S3Spec.Connection.Inline.Host + ":" + strconv.Itoa(port)
}

// get sercret info
func (s *S3ConfigExtractor) GetS3SecretData(ctx context.Context,
	creditial *commonsv1alpha1.Credentials) (accessKey, secretKey string, err error) {
	if creditial == nil {
		err = errors.New("S3 credentials cannot be nil")
		return
	}
	//TODO: credential.Scope
	name := creditial.SecretClass
	if name == "" {
		err = errors.New("S3 secret class cannot be empty")
		return
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: s.Namespace,
			Name:      name,
		},
	}
	err = s.client.GetWithObject(ctx, secret)
	if err != nil {
		return
	}
	accessKey = string(secret.Data[S3AccessKeyName])
	secretKey = string(secret.Data[S3SecretKeyName])
	return
}
