package main

import (
	"testing"
)

func BenchmarkRoutingFirst(b *testing.B) {
	for i := 0; i < b.N; i++ {
		findHandler("install.west.spy.net", "/whatever")
	}
}

func BenchmarkRoutingLast(b *testing.B) {
	for i := 0; i < b.N; i++ {
		findHandler("bleu.west.spy.net",
			"/nging.git/objects/38/0c4d554cd1e1ca08be21389787745f3fc55c09")
	}
}

func BenchmarkRoutingDefault(b *testing.B) {
	for i := 0; i < b.N; i++ {
		findHandler("something", "/whatever")
	}
}
