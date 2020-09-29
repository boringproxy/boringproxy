package main

import (
        "io/ioutil"
	"net/http"
        "errors"
        "strings"
        "encoding/json"
)


func saveJson(data interface{}, filePath string) error {
        jsonStr, err := json.MarshalIndent(data, "", "  ")
        if err != nil {
                return errors.New("Error serializing JSON")
        } else {
                err := ioutil.WriteFile(filePath, jsonStr, 0644)
                if err != nil {
                        return errors.New("Error saving JSON")
                }
        }
        return nil
}


// Looks for auth token in cookie, then header, then query string
func extractToken(tokenName string, r *http.Request) (string, error) {
	tokenCookie, err := r.Cookie(tokenName)

	if err == nil {
		return tokenCookie.Value, nil
	}

	tokenHeader := r.Header.Get(tokenName)

	if tokenHeader != "" {
		return tokenHeader, nil
	}

	authHeader := r.Header.Get("Authorization")

	if authHeader != "" {
		tokenHeader := strings.Split(authHeader, " ")[1]
		return tokenHeader, nil
	}

	query := r.URL.Query()

	queryToken := query.Get(tokenName)
	if queryToken != "" {
		return queryToken, nil
	}

	return "", errors.New("No token found")
}
