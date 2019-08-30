package deepmock

import (
	"errors"

	"go.uber.org/zap"

	"github.com/valyala/fasthttp"
)

type (
	CommonResource struct {
		Code         int         `json:"code"`
		Data         interface{} `json:"data,omitempty"`
		ErrorMessage string      `json:"err_msg,omitempty"`
	}

	ResourceRequestMatcher struct {
		Path   string `json:"path"`
		Method string `json:"method"`
	}

	ResourceRule struct {
		ID        string                        `json:"id,omitempty"`
		Request   *ResourceRequestMatcher       `json:"request"`
		Context   ResourceContext               `json:"context,omitempty"`
		Weight    ResourceWeight                `json:"weight,omitempty"`
		Responses ResourceResponseRegulationSet `json:"responses"`
	}

	ResourceResponseRegulation struct {
		IsDefault bool                      `json:"is_default,omitempty"`
		Filter    *ResourceFilter           `json:"filter,omitempty"`
		Response  *ResourceResponseTemplate `json:"response"`
	}

	ResourceContext map[string]interface{}

	ResourceWeight map[string]ResourceWeightingFactor

	ResourceFilter struct {
		Header ResourceHeaderFilterParameters `json:"header,omitempty"`
		Query  ResourceQueryFilterParameters  `json:"query,omitempty"`
		Body   ResourceBodyFilterParameters   `json:"body,omitempty"`
	}

	ResourceResponseTemplate struct {
		IsTemplate     bool                   `json:"is_template,omitempty"`
		Header         ResourceHeaderTemplate `json:"header,omitempty"`
		StatusCode     int                    `json:"status_code,omitempty"`
		Body           string                 `json:"body,omitempty"`
		B64EncodedBody string                 `json:"base64encoded_body,omitempty"`
	}

	ResourceHeaderFilterParameters map[string]string

	ResourceBodyFilterParameters map[string]string

	ResourceQueryFilterParameters map[string]string

	ResourceHeaderTemplate map[string]string

	ResourceWeightingFactor map[string]uint

	ResourceResponseRegulationSet []*ResourceResponseRegulation
)

func (rrm *ResourceRequestMatcher) check() error {
	if rrm == nil {
		return errors.New("missing request matching")
	}
	if rrm.Path == "" {
		return errors.New("missing path")
	}
	if rrm.Method == "" {
		return errors.New("missing http method")
	}
	return nil
}

func (rmr *ResourceResponseRegulation) check() error {
	if !rmr.IsDefault && rmr.Filter == nil {
		return errors.New("missing filter rule, or set as default response")
	}
	return nil
}

func (mrs ResourceResponseRegulationSet) check() error {
	var d int
	if mrs == nil {
		return errors.New("missing mock response")
	}

	for _, r := range mrs {
		if r.IsDefault {
			d++
		}
		if err := r.check(); err != nil {
			return err
		}
	}
	if d != 1 {
		return errors.New("no default response or provided more than one")
	}
	return nil
}

func HandleMockedAPI(ctx *fasthttp.RequestCtx, next func(error)) {
	re, founded := defaultRuleManager.findExecutor(ctx.Request.URI().Path(), ctx.Request.Header.Method())
	if !founded {
		res := new(CommonResource)
		res.Code = 400
		res.ErrorMessage = "no rule match your request"
		data, _ := json.Marshal(res)
		ctx.Response.Header.SetContentType("application/json")
		ctx.Response.SetBody(data)
		return
	}

	var defaultRegulation *responseRegulation
	for _, regulation := range re.responseRegulations {
		if regulation.isDefault {
			defaultRegulation = regulation
		}
		if !regulation.filter(&ctx.Request) {
			continue
		}

		render(re, regulation.responseTemplate, ctx)
		return
	}

	// 没有任何模板匹配到
	if defaultRegulation == nil {
		res := new(CommonResource)
		res.Code = 400
		res.ErrorMessage = "missing matched response regulation"
		data, _ := json.Marshal(res)
		ctx.Response.Header.SetContentType("application/json")
		ctx.Response.SetBody(data)
		return
	}

	render(re, defaultRegulation.responseTemplate, ctx)
}

func render(re *ruleExecutor, rt *responseTemplate, ctx *fasthttp.RequestCtx) {
	rt.header.CopyTo(&ctx.Response.Header)
	if rt.isTemplate {
		c := re.context
		w := re.weightPicker.dice()
		q := extractQueryAsParams(&ctx.Request)
		f, j := extractBodyAsParams(&ctx.Request)

		rc := renderContext{Context: c, Weight: w, Query: q, Form: f, Json: j}
		if err := rt.render(rc, &ctx.Response); err != nil {
			Logger.Error("failed to render response template", zap.Error(err))
			res := new(CommonResource)
			res.Code = fasthttp.StatusBadRequest
			res.ErrorMessage = err.Error()
			data, _ := json.Marshal(res)
			ctx.Response.SetBody(data)
			return
		}
		return
	}

	ctx.Response.SetBody(rt.body)
}

func HandleCreateRule(ctx *fasthttp.RequestCtx, next func(error)) {
	rule := new(ResourceRule)
	if err := bindBody(ctx, rule); err != nil {
		return
	}

	re, err := defaultRuleManager.createRule(rule)
	res := new(CommonResource)
	if err != nil {
		res.Code = fasthttp.StatusBadRequest
		res.ErrorMessage = err.Error()
	} else {
		res.Code = 200
		res.Data = rule
		rule.ID = re.id()
	}

	data, _ := json.Marshal(res)
	ctx.Response.Header.SetContentType("application/json")
	ctx.Response.SetBody(data)
}

func HandleGetRule(ctx *fasthttp.RequestCtx, next func(error)) {

}

func HandleDeleteRule(ctx *fasthttp.RequestCtx, next func(error)) {

}

func HandleUpdateRule(ctx *fasthttp.RequestCtx, next func(error)) {

}

func HandlePatchRule(ctx *fasthttp.RequestCtx, next func(error)) {

}

func HandleExportRules(ctx *fasthttp.RequestCtx, next func(error)) {

}

func HandleImportRules(ctx *fasthttp.RequestCtx, next func(error)) {

}

func bindBody(ctx *fasthttp.RequestCtx, v interface{}) error {
	if err := json.Unmarshal(ctx.Request.Body(), v); err != nil {
		Logger.Error("failed to parse request body", zap.ByteString("path", ctx.Request.URI().Path()), zap.ByteString("method", ctx.Request.Header.Method()), zap.Error(err))
		ctx.Response.Header.SetContentType("application/json")
		ctx.Response.SetStatusCode(fasthttp.StatusOK)

		res := new(CommonResource)
		res.Code = fasthttp.StatusBadRequest
		res.ErrorMessage = err.Error()
		data, _ := json.Marshal(res)
		ctx.SetBody(data)
		return err
	}
	return nil
}
