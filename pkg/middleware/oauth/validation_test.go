package oauth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestValidateErrors(t *testing.T) {
	testcases := []struct {
		token  string
		err    error
		expLog string
	}{
		// no token
		{"", middleware.ErrInvalidRequest, "invalid format for authorization header"},
		// invalid token
		{"bearer ", middleware.ErrInvalidRequest, "invalid format for authorization header"},
		// invalid jwt
		{"bearer aaa.bbb", middleware.ErrInvalidToken, "jwt token is not of the format hhh.ppp.sss"},
		// invalid jwt parse
		{"bearer aaa.bbb.vvv", middleware.ErrInvalidToken, "Failed to unmarshal jwt header"},
		// invalid claim
		{"bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiYXV" +
			"kIjoiSm9obiBEb2UiLCJhZG1pbiI6dHJ1ZSwiaWF0IjoxNTE2MjM5MDIyfQ.Uaf0IkswiDKIK-zihvB5oK9JrbcXNA1ioKAt-6KI6V6KdmG" +
			"8wWVkLRA5IT0IY9ypInnf7fRx3ieNIodSF08-h8jBXurcjdOvgKBiCl8rNz7mQ_jNDP6ulDSzQAR_wRrLVRs4ObBEWcGYgMwlQ2Vk1EWOkv" +
			"hkxwU9c5_ulDXHD8UMmWy4dM9fiw8Hstjm3zEDPMmQ_jYJ4KCRIWGeDcBTc4MKbkjoa1-zbsKokFYQRqwBzqVkFSbsNlIYZNwkXK6x_nTIg" +
			"WG97bBZCBXTSBnoPoU7_4AcjlSTc6upsdm4anZU8MKZQBHy9nPVZPAIV3ou3qpHxAhe1G1M7eub18mtew", middleware.ErrInvalidToken,
			"Failed to parse token"},
		// invalid request
		{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiYXVkIjoiSm9" +
			"obiBEb2UiLCJhZG1pbiI6dHJ1ZSwiaWF0IjoxNTE2MjM5MDIyfQ.A8FnCpeKccTlE7gg8oebcjepg_O6DhcYcyq923low28", middleware.ErrInvalidRequest,
			"invalid format for authorization header"},
		// invalid modulus
		{"bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMzA9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwi" +
			"YWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.AEFESFUc0QvP7T_KQt_E-18YG9WVwOUYGVTHokPFdc4", middleware.ErrInvalidToken,
			"Error while getting public key"},
		// invalid signature
		{"bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiYXV" +
			"kIjoiSm9obiBEb2UiLCJhZG1pbiI6dHJ1ZSwiaWF0IjoxNTE2MjM5MDIyfQ.Uaf0IkswiDKIK-zihvB5oK9JrbcXNA1ioKAt-6KI6V6Kdm" +
			"G8wWVkLRA5IT0IY9ypInnf7fRx3ieNIodSF08-h8jBXurcjdOvgKBiCl8rNz7mQ_jNDP6ulDSzQAR_wRrLVRs4ObBEWcGYgMwlQ2Vk1EWOk" +
			"vhkxwU9c5_ulDXHD8UMmWy4dM9fiw8Hstjm3zEDPMmQ_jYJ4KCRIWGeDcBTc4MKbkjoa1-zbsKokFYQRqwBzqVkFSbsNlIYZNwkXK6x_nTI" +
			"gWG97bBZCBXTSBnoPoU7_4AcjlSTc6upsdm4anZU8MKZQBHy9nPVZPAIV3ou3qpHxAhe1G1M7eub18mtew", middleware.ErrInvalidToken,
			"Failed to parse token"},
		// invalid algorithm
		{"bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiYXV" +
			"kIjoiSm9obiBEb2UiLCJhZG1pbiI6dHJ1ZSwiaWF0IjoxNTE2MjM5MDIyfQ.A8FnCpeKccTlE7gg8oebcjepg_O6DhcYcyq923low28",
			middleware.ErrInvalidToken, "Failed to parse token"},
	}

	for i, v := range testcases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
		req.Header.Set("Authorization", v.token)

		key := PublicKey{
			ID:             "2011-04-30==",
			Alg:            "RS256",
			Type:           "RSA",
			Modulus:        "^",
			PublicExponent: "AQAB",
			PrivateExponent: `X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAK
            NLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TN
            wm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8
            fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q`,
		}

		oAuth := OAuth{
			options: Options{
				ValidityFrequency: 10,
				JWKPath:           getTestServerURL(),
			},
			cache: PublicKeyCache{
				publicKeys: PublicKeys{Keys: []PublicKey{key}},
				mu:         sync.RWMutex{},
			},
		}

		_, err := oAuth.Validate(logger, req)
		if !errors.Is(err, v.err) {
			t.Errorf("Testcase[%v] Failed, validate() = %v, \nwant %v", i+1, err, v.err)
		}

		if !strings.Contains(b.String(), v.expLog) {
			t.Errorf("Testcase[%v] failed: Expected: %v, Got: %v", i, v.expLog, b.String())
		}
	}
}

func TestValidateSuccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
	req.Header.Set("Authorization",
		"bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmF"+
			"tZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.B5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92"+
			"o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk"+
			"-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57"+
			"C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA")

	expectedToken := &jwt.Token{Raw: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0" +
		"NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.B5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgd" +
		"LxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJM" +
		"Nk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8a" +
		"S51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA", Method: jwt.SigningMethodRS256,
		Header: map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": "2011-04-29=="},
		Claims: jwt.MapClaims{"iat": 1.516239022e+09, "sub": "1234567890", "admin": true, "name": "John Doe"},
		Signature: "B5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxl" +
			"S_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNg" +
			"Geter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp" +
			"4plzdIMnjIXXp8kuviYl7feA", Valid: true}

	key := PublicKey{
		ID:             "2011-04-30==",
		Alg:            "RS256",
		Type:           "RSA",
		Modulus:        "^",
		PublicExponent: "AQAB",
		PrivateExponent: `X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAK
            NLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TN
            wm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fk
            Fzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q`,
	}

	oAuth := OAuth{
		options: Options{
			ValidityFrequency: 10,
			JWKPath:           getTestServerURL(),
		},
		cache: PublicKeyCache{
			publicKeys: PublicKeys{Keys: []PublicKey{key}},
			mu:         sync.RWMutex{},
		},
	}

	err := oAuth.invalidateCache(log.NewLogger())
	if err != nil {
		t.Error(err)
	}

	resp, err := oAuth.Validate(log.NewLogger(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v\n", err)
	}

	if !reflect.DeepEqual(resp, expectedToken) {
		t.Errorf("Failed. Got : %v\n Expected : %v\n", resp, expectedToken)
	}
}

func getTestServerURL() string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trap := PublicKeys{Keys: []PublicKey{}}
		_ = json.Unmarshal([]byte(validJWKSet()), &trap)
		jsonResp, _ := json.Marshal(trap)
		_, _ = w.Write(jsonResp)
	}))

	return server.URL
}

func TestValidate_RawStdEncoding_Header(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/dummy", nil)
	//nolint:lll // token value is long
	req.Header.Set("Authorization", "bearer eyJhbGciOiJSUzI1NiIsImprdSI6Imh0dHBzOi8vYXBpLnpvcHNtYXJ0LmNvbS92MS8ud2VsbC1rbm93bi9qd2tzLmpzb24iLCJraWQiOiJCbWl4SjN6eUVObFQxYjB6Tm1pbWtRPT0iLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJ6b3BzbWFydC10ZXN0LTBlM2NjMjc5NzY2M2Y2ZjI5MmY1NjhkZDU0YjhkOWQ5IiwiZXhwIjoxOTg5NDgxMTA3LCJpYXQiOjE1ODk0NzkzMDIsImlzcyI6ImFwaS1zYi56b3BzbWFydC5jb20iLCJzdWIiOiI5MTQ3ZTFiOS1jNzgxLTU5ZmUtODJkZS1mMjA1NjJmMTZmMWMiLCJzY29wZSI6IiIsImF1dGhBdCI6MTU4OTQ3OTMwNzA5NDA5NzAwMCwiYXpwIjoiem9wc21hcnQtdGVzdC0wZTNjYzI3OTc2NjNmNmYyOTJmNTY4ZGQ1NGI4ZDlkOSJ9.odhLuiNd6C-TxBeTikzUBF8NF7x5j9e2eeqVBkgFYaJ8q0Rq-JPDf4p4pWFDi7eCDoQRPQY8KXSHxilikkrQqfXoCsGr09vPiQNL2cFn71ovxTR21bH0SafWKYlKefcuP7ocLOCb2JX38TfOP3pqc6tPRzDDgi9v3F818w9lYjyojyRiJ5wz7D6jFYWOG4wuUeGgCS7TOXRPLeAr2WsNkZd5v5_b_69sElmk9LjedOIBpsJBT0RYsELDiUOermOUEdcmQgZvLvi1k2NiVJCJ88rl-bWLSXE-PPVrUxhv4B-zKbPowDXJRyVyzK-3qKQKFfZQlmfhb4CrXanEoWhG7Q")

	resp, err := getJWT(log.NewLogger(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v\n", err)
	}

	//nolint:lll // payload value is long
	expectedToken := JWT{
		payload: "eyJhdWQiOiJ6b3BzbWFydC10ZXN0LTBlM2NjMjc5NzY2M2Y2ZjI5MmY1NjhkZDU0YjhkOWQ5IiwiZXhwIjoxOTg5NDgxMTA3LCJpYXQiOjE1ODk0NzkzMDIsImlzcyI6ImFwaS1zYi56b3BzbWFydC5jb20iLCJzdWIiOiI5MTQ3ZTFiOS1jNzgxLTU5ZmUtODJkZS1mMjA1NjJmMTZmMWMiLCJzY29wZSI6IiIsImF1dGhBdCI6MTU4OTQ3OTMwNzA5NDA5NzAwMCwiYXpwIjoiem9wc21hcnQtdGVzdC0wZTNjYzI3OTc2NjNmNmYyOTJmNTY4ZGQ1NGI4ZDlkOSJ9",
		header: header{
			Algorithm: "RS256",
			Type:      "JWT",
			URL:       "https://api.gofr.dev/v1/.well-known/jwks.json",
			KeyID:     "BmixJ3zyENlT1b0zNmimkQ==",
		},
		signature: "odhLuiNd6C-TxBeTikzUBF8NF7x5j9e2eeqVBkgFYaJ8q0Rq-JPDf4p4pWFDi7eCDoQRPQY8KXSHxilikkrQqfXoCsGr09vPiQNL2cFn71ovxTR21bH0SafWKYlKefcuP7ocLOCb2JX38TfOP3pqc6tPRzDDgi9v3F818w9lYjyojyRiJ5wz7D6jFYWOG4wuUeGgCS7TOXRPLeAr2WsNkZd5v5_b_69sElmk9LjedOIBpsJBT0RYsELDiUOermOUEdcmQgZvLvi1k2NiVJCJ88rl-bWLSXE-PPVrUxhv4B-zKbPowDXJRyVyzK-3qKQKFfZQlmfhb4CrXanEoWhG7Q",
		token:     "eyJhbGciOiJSUzI1NiIsImprdSI6Imh0dHBzOi8vYXBpLnpvcHNtYXJ0LmNvbS92MS8ud2VsbC1rbm93bi9qd2tzLmpzb24iLCJraWQiOiJCbWl4SjN6eUVObFQxYjB6Tm1pbWtRPT0iLCJ0eXAiOiJKV1QifQ.eyJhdWQiOiJ6b3BzbWFydC10ZXN0LTBlM2NjMjc5NzY2M2Y2ZjI5MmY1NjhkZDU0YjhkOWQ5IiwiZXhwIjoxOTg5NDgxMTA3LCJpYXQiOjE1ODk0NzkzMDIsImlzcyI6ImFwaS1zYi56b3BzbWFydC5jb20iLCJzdWIiOiI5MTQ3ZTFiOS1jNzgxLTU5ZmUtODJkZS1mMjA1NjJmMTZmMWMiLCJzY29wZSI6IiIsImF1dGhBdCI6MTU4OTQ3OTMwNzA5NDA5NzAwMCwiYXpwIjoiem9wc21hcnQtdGVzdC0wZTNjYzI3OTc2NjNmNmYyOTJmNTY4ZGQ1NGI4ZDlkOSJ9.odhLuiNd6C-TxBeTikzUBF8NF7x5j9e2eeqVBkgFYaJ8q0Rq-JPDf4p4pWFDi7eCDoQRPQY8KXSHxilikkrQqfXoCsGr09vPiQNL2cFn71ovxTR21bH0SafWKYlKefcuP7ocLOCb2JX38TfOP3pqc6tPRzDDgi9v3F818w9lYjyojyRiJ5wz7D6jFYWOG4wuUeGgCS7TOXRPLeAr2WsNkZd5v5_b_69sElmk9LjedOIBpsJBT0RYsELDiUOermOUEdcmQgZvLvi1k2NiVJCJ88rl-bWLSXE-PPVrUxhv4B-zKbPowDXJRyVyzK-3qKQKFfZQlmfhb4CrXanEoWhG7Q",
	}

	assert.Equal(t, resp, expectedToken)
}

func TestGetJWT(t *testing.T) {
	validJwtParts := []string{
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9",
		"eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0",
		//nolint
		"5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA",
	}
	invalidJWTParts := make([]string, 3)
	copy(invalidJWTParts, validJwtParts)
	invalidJWTParts[0] = "eyJhbGciOiJSUzI1NiIsImtpZDEiOiIyMDExLTA0LTI5PT0ifQ=="
	testcases := []struct {
		jwtToken   string
		JWT        JWT
		error      error
		logMessage string
	}{
		{"", JWT{}, middleware.ErrInvalidRequest, "invalid format for authorization header"},
		{"aksabdjkd", JWT{}, middleware.ErrInvalidRequest, "invalid format for authorization header"},
		{"bear aksabdjkd", JWT{}, middleware.ErrInvalidRequest, "invalid format for authorization header"},
		{"bearer abc", JWT{}, middleware.ErrInvalidToken, "jwt token is not of the format hhh.ppp.sss"},
		{"bearer abc.def", JWT{}, middleware.ErrInvalidToken, "jwt token is not of the format hhh.ppp.sss"},
		{"bearer abc.def.ghi.jkl", JWT{}, middleware.ErrInvalidToken, "jwt token is not of the format hhh.ppp.sss"},
		{"bearer abc.def.ghi", JWT{}, middleware.ErrInvalidToken, "Failed to unmarshal jwt header"},
		{"bearer " + strings.Join(invalidJWTParts, "."), JWT{}, middleware.ErrInvalidToken, "Failed to decode jwt header"},

		//nolint
		{"bearer " + strings.Join(validJwtParts, "."),
			JWT{
				payload: validJwtParts[1],
				header: header{
					Algorithm: "RS256",
					Type:      "JWT",
					URL:       "",
					KeyID:     "2011-04-29==",
				},
				signature: validJwtParts[2],
				token:     strings.Join(validJwtParts, "."),
			}, nil, ""},
	}

	for i, testCase := range testcases {
		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", testCase.jwtToken)
		got, err := getJWT(logger, request)

		assert.Equal(t, testCase.error, err)
		assert.Equal(t, testCase.JWT, got, i)

		if !strings.Contains(b.String(), testCase.logMessage) {
			t.Errorf("Testcase[%v] failed: Expected: %v, Got: %v", i, testCase.logMessage, b.String())
		}
	}
}
