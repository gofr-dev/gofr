package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"gofr.dev/pkg/log"
	"gofr.dev/pkg/middleware"
)

func Test_getPublicKey(t *testing.T) {
	testKey := PublicKey{ID: "Z4Fd3mskIH88irt7LB5c6g==", Alg: "RS256", Type: "RSA", Use: "sig", Operations: []string{"verify"},
		Modulus: "xZY4JS80m5pkZaXz_SPuU0MvXkPo65XfcygDMk5fGlTV_TWxQ", PublicExponent: "AQAB"}

	testcases := []struct {
		redisKey string
		key      *PublicKey
	}{ // key exist in cache
		{"Z4Fd3mskIH88irt7LB5c6g==", &testKey},
		// key not found
		{"some_random_key", &PublicKey{}},
	}

	for i := range testcases {
		keyCache := PublicKeyCache{
			publicKeys: PublicKeys{Keys: []PublicKey{*testcases[i].key}},
			mu:         sync.RWMutex{},
		}

		outputKeys := keyCache.publicKeys.Get(testcases[i].redisKey)

		if !reflect.DeepEqual(outputKeys, testcases[i].key) {
			t.Errorf("Failed testcase %d , Expected KEY : %v, Got : %v", i+1, testcases[i].key, outputKeys)
		}
	}
}

func TestGetPublicKeyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		_, _ = w.Write([]byte(`"hello"`))
	}))

	o := OAuth{
		options: Options{
			ValidityFrequency: 0,
			JWKPath:           ts.URL,
		},
		cache: PublicKeyCache{},
	}

	key := o.cache.publicKeys.Get("")
	expKey := PublicKey{}

	if reflect.DeepEqual(key, expKey) {
		t.Errorf("Expected nil, Got : %v", key)
	}
}

func TestPublicKeys_GetRSAKey(t *testing.T) {
	testKey := PublicKey{
		ID:   "2011-04-30==",
		Alg:  "RS256",
		Type: "RSA",
		Modulus: "vbQ245FQ0KsapUHkXFIzD7IdjoV2OjQTQ3fFhjsFVUv-OOPbF1sjtRjn2qsmJ26K-_VVEtFA_zcaxwETVELO3alBKdL-" +
			"LrZeRaeGktylbU4MN_vfnpsd1ors7q0OwIlaHvUej1oDy1rTRqx_n9R4IsKAxH7PGB6fqe0jmjhMLi9lqIDskZC4g7jdIOztRsZ8fHEG" +
			"2kEZK67zR0TyWh27gD4_H1pZFUTm_JbTQkGMDAKWqy3h3N60UCDzexFj6SOqltK-waNIkxQToR2ub-bLV5XUpg9fZSY16qVxxL8d9Ohd" +
			"EoFrl-uIq16q6WDzLKFeegj1bu2mXX_gahJD1-fzHQ",
		PublicExponent: "AQAB",
		PrivateExponent: `X4cTteJY_gn4FYPsXB8rdXix5vwsg1FLN5E3EaG6RJoVH-HLLKD9M7dx5oo7GURknchnrRweUkC7hT5fJLM0WbFAK
            NLWY2vv7B6NqXSzUvxT0_YSfqijwp3RTzlBaCxWp4doFk5N2o8Gy_nHNKroADIkJ46pRUohsXywbReAdYaMwFs9tv8d_cPVY3i07a3t8MN6TN
            wm0dSawm9v47UiCl3Sk5ZiG7xojPLu4sbg1U2jx4IBTNBznbJSzFHK66jT8bgkuqsk0GjskDJk19Z4qwjwbsnn4j2WBii3RL-Us2lGVkY8fk
            Fzme1z0HbIkfz0Y6mqnOYtqc0X4jfcKoAC8Q`,
	}
	keyCache := PublicKeyCache{}
	keyCache.publicKeys.Keys = []PublicKey{testKey}

	rsaKey, err := testKey.getRSAPublicKey()
	if err != nil {
		t.Error(err)
	}

	if rsaKey.N == nil {
		t.Error("Should have got RSA Key")
	}

	// checking for getting rsa from cache
	testKey2 := testKey
	testKey2.rsaPublicKey = rsaKey

	keyCache2 := PublicKeyCache{}
	keyCache2.publicKeys.Keys = []PublicKey{testKey}

	rsaKey2, err := testKey2.getRSAPublicKey()
	if err != nil {
		t.Error(err)
	}

	if rsaKey2.N == nil {
		t.Error("Should have got RSA Key")
	}
}

func Test_LoadJWK(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(testLoadJWKHandler))

	var k PublicKeys
	if err := json.Unmarshal([]byte(validJWKSet()), &k); err != nil {
		t.Errorf("Encountered error while initializing valid JWK")
		return
	}

	testCases := []struct {
		JWKPath string
		keys    []PublicKey
		error   error
	}{
		{testServer.URL + "/BAD_URL", nil, middleware.ErrServiceDown},
		{testServer.URL + "/BAD_RESP", nil, middleware.ErrServiceDown},
		{testServer.URL + "/BAD_JSON", nil, middleware.ErrServiceDown},
		{testServer.URL + "/SUCCESS", k.Keys, nil},
	}
	for _, testCase := range testCases {
		oAuth := New(log.NewLogger(), Options{
			ValidityFrequency: 0,
			JWKPath:           testCase.JWKPath,
		})
		keys, err := oAuth.loadJWK(log.NewLogger())

		if err != testCase.error {
			t.Errorf("Expected error: %v, got: %v", testCase.error, err)
		}

		if !reflect.DeepEqual(keys, testCase.keys) {
			t.Errorf("Expected keys: %v, got: %v", testCase.keys, keys)
		}
	}
}

func testLoadJWKHandler(w http.ResponseWriter, r *http.Request) {
	response := ""

	switch r.URL.Path {
	case "/BAD_URL":
	case "/BAD_RESP":
	case "/BAD_JSON":
		response = "some random string which is not in JSON format"
	default:
		response = validJWKSet()
	}

	_, _ = w.Write([]byte(response))
}
