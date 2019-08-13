/*
 * Copyright 2019 Azz. All rights reserved.
 * Use of this source code is governed by a GPL-3.0
 * license that can be found in the LICENSE file.
 */

package kinoko_web

import (
	"encoding/json"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strings"
)

type RequestCtx struct {
	QueryString    map[string][]string
	PathVariable   map[string]string
	Request        *http.Request
	Form           *multipart.Form
	ResponseWriter http.ResponseWriter
}

func NewRequestCtx(queryString map[string][]string, pathVariable map[string]string, request *http.Request, form *multipart.Form, responseWriter http.ResponseWriter) *RequestCtx {
	return &RequestCtx{
		QueryString:    queryString,
		PathVariable:   pathVariable,
		Request:        request,
		Form:           form,
		ResponseWriter: responseWriter,
	}
}

func (c *RequestCtx) ParseBody(dst interface{}) error {
	ct := c.Request.Header.Get("Content-Type")
	if !strings.EqualFold(ct, "application/json") {
		logger.Info("content-type: '%s' is not supported yet\n", ct)
		return nil
	}

	//TODO support url-encoded

	bytes, e := ioutil.ReadAll(c.Request.Body) //Note that ReadAll is not safe for oom attack
	defer func() {
		_ = c.Request.Body.Close()
	}()

	if e != nil {
		return e
	}

	e = json.Unmarshal(bytes, dst)
	if e != nil {
		return e
	}
	return nil
}
