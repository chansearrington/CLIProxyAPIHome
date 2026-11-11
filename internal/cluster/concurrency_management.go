package cluster

import (
	"context"
	"fmt"
	"strings"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"gorm.io/gorm"
)

// AuthConcurrencyPatchRequest combines an optional locked-current auth mutation with a presence-aware limiter policy update.
type AuthConcurrencyPatchRequest struct {
	CredentialID          string
	MutateAuth            func(*coreauth.Auth) (bool, error)
	PolicyPatch           ConcurrencyPolicyPatch
	ExpectedPolicyVersion *int64
}

// PatchAuthAndConcurrency atomically applies auth metadata and limiter policy changes.
func (r *Repository) PatchAuthAndConcurrency(ctx context.Context, request AuthConcurrencyPatchRequest) error {
	credentialID := strings.TrimSpace(request.CredentialID)
	if credentialID == "" {
		return ErrConcurrencyCredentialNotFound
	}
	if errValidate := validateConcurrencyPolicyPatchModelKeys(request.PolicyPatch); errValidate != nil {
		return errValidate
	}
	db, errDB := r.database()
	if errDB != nil {
		return errDB
	}
	return withConcurrencyTransaction(ctx, db, func(tx *gorm.DB) error {
		policy := CredentialConcurrencyPolicy{}
		if errPolicy := r.patchCredentialConcurrencyPolicyTx(ctx, tx, credentialID, request.PolicyPatch, request.ExpectedPolicyVersion, &policy); errPolicy != nil {
			return errPolicy
		}
		if request.MutateAuth == nil {
			return nil
		}
		var errPatch error
		_, _, _, errMutate := (&Repository{db: tx}).MutateAuth(ctx, credentialID, "update", func(current *coreauth.Auth) bool {
			changed := false
			changed, errPatch = request.MutateAuth(current)
			if current.ID != credentialID || current.Index != credentialID {
				errPatch = fmt.Errorf("auth credential id does not match concurrency credential")
			}
			return changed && errPatch == nil
		})
		if errMutate != nil {
			return errMutate
		}
		return errPatch
	})
}
