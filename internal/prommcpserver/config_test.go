package prommcpserver

import (
	"testing"
)

func TestConfigToolName(t *testing.T) {
	c := &Config{ToolPrefix: "stg"}
	if c.ToolName("execute_query") != "stg_execute_query" {
		t.Fatal()
	}
	c2 := &Config{}
	if c2.ToolName("execute_query") != "execute_query" {
		t.Fatal()
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "stdio"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{MCPTransport: "stdio"}).Validate(); err == nil {
		t.Fatal("want error")
	}
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "bad"}).Validate(); err == nil {
		t.Fatal("want error")
	}
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "http", MCPBindPort: 0}).Validate(); err == nil {
		t.Fatal("want error")
	}
}
