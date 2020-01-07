package main

import (
	"errors"

	"github.com/valyala/fasthttp"
)

const prefix = "Bearer "

type tokenAuth struct {
	token string
}

var defaultToken = []byte("token")
var AuthHeader = []byte("Authorization")

func generateToken() []byte {
	return defaultToken
}

func loginHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.SetBody(generateToken())
	return
}

func tokenVerify(ctx *fasthttp.RequestCtx) bool {
	token, err := getToken(ctx)
	if err != nil {
		return false
	}
	return equalFold(token, defaultToken)
}

// "authorization": "Bearer " + t.token,

func getToken(ctx *fasthttp.RequestCtx) ([]byte, error) {
	authByte := ctx.Request.Header.PeekBytes(AuthHeader)
	if len(authByte) == 0 {
		return nil, errors.New("no token")
	}

	return authByte, nil
}

func equalFold(b, s []byte) (equals bool) {
	n := len(b)
	if n != len(s) {
		equals = false
	} else {
		equals = true
		for i := 0; i < n; i++ {
			if b[i]|0x20 != s[i]|0x20 {
				equals = false
				break
			}
		}
	}
	return
}

// if !strings.HasPrefix(auth, prefix) {
// 	return "", errors.New(`missing "Bearer " prefix in "Authorization" header`)
// }
//
// token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
// if len(token) == 0 {
// 	return "", errors.New("no token string")
// }
