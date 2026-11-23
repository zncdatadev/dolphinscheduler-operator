package v1alpha1

type AuthenticationSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	AuthenticationClass string `json:"authenticationClass,omitempty"`

	// +kubebuilder:validation:Optional
	Oidc *OidcCredentialSecretSpec `json:"oidc,omitempty"`
}

type OidcCredentialSecretSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Secret string `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	ExtraScopes []string `json:"extraScopes,omitempty"`
}
