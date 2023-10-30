package oauth

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/log"
)

func validJWKSet() string {
	//nolint
	return `{
    "keys": [
        {
            "kty": "RSA",
            "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
            "e": "AQAB",
            "d": "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q",
            "p": "83i-7IvMGXoMXCskv73TKr8637FiO7Z27zv8oj6pbWUQyLPQBQxtPVnwD20R-60eTDmD2ujnMt5PoqMrm8RfmNhVWDtjjMmCMjOpSXicFHj7XOuVIYQyqVWlWEh6dN36GVZYk93N8Bc9vY41xy8B9RzzOGVQzXvNEvn7O0nVbfs",
            "q": "3dfOR9cuYq-0S-mkFLzgItgMEfFzB2q3hWehMuG0oCuqnb3vobLyumqjVZQO1dIrdwgTnCdpYzBcOfW5r370AFXjiWft_NGEiovonizhKpo9VVS78TzFgxkIdrecRezsZ-1kYd_s1qDbxtkDEgfAITAG9LUnADun4vIcb6yelxk",
            "dp": "G4sPXkc6Ya9y8oJW9_ILj4xuppu0lzi_H7VTkS8xj5SdX3coE0oimYwxIi2emTAue0UOa5dpgFGyBJ4c8tQ2VF402XRugKDTP8akYhFo5tAA77Qe_NmtuYZc3C3m3I24G2GvR5sSDxUyAN2zq8Lfn9EUms6rY3Ob8YeiKkTiBj0",
            "dq": "s9lAH9fggBsoFR8Oac2R_E2gw282rT2kGOAhvIllETE1efrA6huUUvMfBcMpn8lqeW6vzznYY5SSQF7pMdC_agI3nG8Ibp1BUb0JUiraRNqUfLhcQb_d9GF4Dh7e74WbRsobRonujTYN1xCaP6TO61jvWrX-L18txXw494Q_cgk",
            "qi": "GyM_p6JrXySiz1toFgKbWV-JdI3jQ4ypu9rbMWx3rQJBfmt0FoYzgUIZEVFEcOqwemRN81zoDAaa-Bk0KWNGDjJHZDdDmFhW3AN7lI-puxk_mHZGJ11rxyR8O55XLSe3SPmRfKwZI6yU24ZxvQKFYItdldUKGzO6Ia6zTKhAVRU",
            "alg": "RS256",
            "kid": "2011-04-29=="
        }
    ]
}`
}

type MockHandlerForOAuth struct{}

func (r *MockHandlerForOAuth) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Response == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`"if you see this, the jwt was valid...."`))

		return
	}

	w.WriteHeader(req.Response.StatusCode)
	resp, err := io.ReadAll(req.Response.Body)

	if err == nil {
		_, _ = w.Write(resp)
	}
}

type MockedResponseWriter struct {
	HeaderMap http.Header
	Body      *bytes.Buffer
	Code      int
}

func (rw *MockedResponseWriter) Header() http.Header {
	m := rw.HeaderMap
	if m == nil {
		m = make(http.Header)
		rw.HeaderMap = m
	}

	return m
}

func (rw *MockedResponseWriter) Write(buf []byte) (int, error) {
	rw.Body = new(bytes.Buffer)
	rw.Body.Write(buf)

	return len(buf), nil
}

func (rw *MockedResponseWriter) WriteHeader(statusCode int) {
	rw.Code = statusCode
}

type MultipleErrors struct {
	StatusCode int               `json:"-" xml:"-"`
	Errors     []errors.Response `json:"errors" xml:"errors"`
}

//nolint:gocognit // breaking down function will reduce readability
func Test_Middleware_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	testcases := []struct {
		jwtEndpoint   string
		jwtToken      string
		expectedError string // to compare with errors.Response
		expectedCode  int
	}{
		{"http://localhost:8080/jwt-token", "", "invalid_request", http.StatusUnauthorized},
		{"http://localhost:8080/jwt-token", "bear abc.def.ghi", "invalid_request", http.StatusUnauthorized},
		{"http://localhost:8080/jwt-token", "bearer abc.def", "invalid_token", http.StatusUnauthorized},
		//nolint
		{"http://localhost:8080/jwt-bad-token", "bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA",
			"invalid_token", http.StatusUnauthorized},
		//nolint
		{"random-url", "bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA",
			"invalid_token", http.StatusUnauthorized},
		{"", "", "", http.StatusOK},
		//nolint
		{"http://localhost:8080/jwt-token", "bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA",
			"invalid_token", http.StatusUnauthorized},
	}

	for i := range testcases {
		t.Setenv("JWKS_ENDPOINT", testcases[i].jwtEndpoint)

		request := httptest.NewRequest(http.MethodGet, "/auth", nil)
		request.Header.Set("Authorization", testcases[i].jwtToken)

		b := new(bytes.Buffer)
		logger := log.NewMockLogger(b)

		w := new(MockedResponseWriter)
		oAuthOptions := Options{JWKPath: os.Getenv("JWKS_ENDPOINT"), ValidityFrequency: 10}
		handler := Auth(logger, oAuthOptions)(&MockHandlerForOAuth{})
		handler.ServeHTTP(w, request)

		if w.Code != testcases[i].expectedCode {
			t.Errorf("Failed testcase %d , Expected STATUS CODE : %v, Got : %v", i+1, testcases[i].expectedCode, w.Code)
		}

		errResp := MultipleErrors{}
		_ = json.Unmarshal(w.Body.Bytes(), &errResp)

		// Check if error is being logged
		if errResp.Errors != nil && !strings.Contains(b.String(), testcases[i].expectedError) {
			t.Errorf("Middleware Error is not logged")
		}

		// the first check is done for cases where an error is not generated
		if errResp.Errors != nil && testcases[i].expectedError != "" && errResp.Errors[0].Code != testcases[i].expectedError {
			t.Errorf("Failed testcase %d , Expected  : %v, Got : %v", i+1, testcases[i].expectedError, errResp.Errors[0].Code)
		}
	}
}

func Test_Middleware_Success(t *testing.T) {
	testcases := []struct {
		jwtToken     string
		kid          string
		expectedBody string
		expectedCode int
		target       string
	}{
		{
			//nolint
			"bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.B5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA",
			"jwt-2011-04-29==",
			`"if you see this, the jwt was valid...."`,
			http.StatusOK,
			"/auth",
		},
		{
			//nolint
			"",
			"",
			`"if you see this, the jwt was valid...."`,
			http.StatusOK,
			"/auth/.well-known/heartbeat",
		},
	}

	for i := range testcases {
		request := httptest.NewRequest(http.MethodGet, testcases[i].target, nil)

		if testcases[i].jwtToken != "" {
			request.Header.Set("Authorization", testcases[i].jwtToken)
		}

		w := new(MockedResponseWriter)

		options := Options{
			ValidityFrequency: 10,
			JWKPath:           getTestServerURL(),
		}
		handler := Auth(log.NewLogger(), options)(&MockHandlerForOAuth{})
		handler.ServeHTTP(w, request)

		if w.Code != testcases[i].expectedCode {
			t.Errorf("Failed testcase %d , Expected STATUS CODE : %v, Got : %v", i+1, testcases[i].expectedCode, w.Code)
		}

		body := strings.TrimSpace(w.Body.String())
		if body != testcases[i].expectedBody {
			t.Errorf("Failed testcase %d , Expected KEY : %v, Got : %v", i+1, testcases[i].expectedBody, body)
		}
	}
}

func Test_generateRsaPublicKey(t *testing.T) {
	testcases := []struct {
		pubKey PublicKey
		expErr error
	}{
		//nolint
		{PublicKey{"2011-04-29==", "RS256", "RSA", "", nil, "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"AQAB", "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q", rsa.PublicKey{}},
			nil},
		//nolint
		{PublicKey{"2011-04-29==", "RS256", "RSA", "", nil, "^", "AQAB", "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q", rsa.PublicKey{}},
			base64.CorruptInputError(0)},
		//nolint
		{PublicKey{"2011-04-29==", "RS256", "RSA", "", nil, "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw", "^",
			"X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q", rsa.PublicKey{}},
			base64.CorruptInputError(0)},
		//nolint
		{PublicKey{"2011-04-29==", "RS256", "RSA", "", nil, "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
			"AQABEEWDWDEWEWECASAS", "X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAKNLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TNwm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fkFzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q", rsa.PublicKey{}},
			nil},
	}

	for tc := range testcases {
		_, err := generateRSAPublicKey(&testcases[tc].pubKey)
		if !reflect.DeepEqual(err, testcases[tc].expErr) {
			t.Errorf("Testcase[%v] failed: Expected: %v, Got: %v", tc, testcases[tc].expErr, err)
		}
	}
}

func Test_RequestClone(t *testing.T) {
	//nolint
	jwtToken := "bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6IjIwMTEtMDQtMjk9PSJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.B5C9tz71T-PjyoMH-gv198iNFguDZ5SpVcwrgdLxU83A92o1tsJWh8_7Zm6ulMUupNEAzGD69DB077j01nXz6ut5XtnXWE50HNTxlS_19zndpPxqFcKnWyoArip5A1MCgQjKQ3exwZc7aFQwgBXvJMNk-5N4od_bUMGvOb0q3ApbfzbwIt94daToPjhfLy4xf8UoNhh_Lq14CNHCZXNgGeter5TvnHnDBN4oDfw6nziKdJnslNkUJ2hHsqp8VObUK57C8aS51x2UiOwTJ1NqDv0PFVgRbC7ncFZG6M87x9BGTwB0XvraXYU7Zimewp4plzdIMnjIXXp8kuviYl7feA"
	target := "/auth"

	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Authorization", jwtToken)

	w := new(MockedResponseWriter)
	options := Options{
		ValidityFrequency: 10,
		JWKPath:           getTestServerURL(),
	}

	handler := Auth(log.NewLogger(), options)(&MockHandlerForOAuth{})
	handler.ServeHTTP(w, request)

	val := request.Context().Value(JWTContextKey("claims"))
	if val == nil {
		t.Errorf("Test case failed. Expected: %v, got: %v", val, nil)
	}
}
