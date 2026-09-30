package oidc

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/services/auth/internal/constants"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

func GetNonce(w http.ResponseWriter, _ *http.Request) {
	nonce, err := oidchelper.GenerateAndStoreNonce()
	if err != nil {
		shared.HandleError(w, err, http.StatusInternalServerError,
			fmt.Sprintf(string(constants.ErrOIDCNonceStoreFailed), err))
		return
	}
	shared.SendSuccessResponse(w, string(constants.SuccessOIDCNonceGenerated), NonceResponse{Nonce: nonce})
}
