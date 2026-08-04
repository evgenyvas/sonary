package foobar2000

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// http GET
func (p *Player) get(path string, out any) error {
	resp, err := p.client.Get(p.cfg.FoobarAPIUrl + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%d %s", resp.StatusCode, b)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// http POST
func (p *Player) post(path string, body any) error {
	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		p.cfg.FoobarAPIUrl+path,
		reader,
	)
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%d %s", resp.StatusCode, b)
	}

	return nil
}
