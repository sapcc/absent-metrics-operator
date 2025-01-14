/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	context "context"

	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gentype "k8s.io/client-go/gentype"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

// TokenReviewsGetter has a method to return a TokenReviewInterface.
// A group's client should implement this interface.
type TokenReviewsGetter interface {
	TokenReviews() TokenReviewInterface
}

// TokenReviewInterface has methods to work with TokenReview resources.
type TokenReviewInterface interface {
	Create(ctx context.Context, tokenReview *authenticationv1.TokenReview, opts metav1.CreateOptions) (*authenticationv1.TokenReview, error)
	TokenReviewExpansion
}

// tokenReviews implements TokenReviewInterface
type tokenReviews struct {
	*gentype.Client[*authenticationv1.TokenReview]
}

// newTokenReviews returns a TokenReviews
func newTokenReviews(c *AuthenticationV1Client) *tokenReviews {
	return &tokenReviews{
		gentype.NewClient[*authenticationv1.TokenReview](
			"tokenreviews",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *authenticationv1.TokenReview { return &authenticationv1.TokenReview{} },
			gentype.PrefersProtobuf[*authenticationv1.TokenReview](),
		),
	}
}
