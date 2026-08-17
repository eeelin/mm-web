package main

import "testing"

func TestModemState(t *testing.T) {
	if got := modemState(8); got != "已注册" {
		t.Fatalf("modemState(8) = %q, want 已注册", got)
	}
	if got := modemState(99); got != "未知状态" {
		t.Fatalf("modemState(99) = %q, want 未知状态", got)
	}
}

func TestAccessTechnology(t *testing.T) {
	if got := accessTechnology(1 << 14); got != "4G LTE" {
		t.Fatalf("LTE technology = %q, want 4G LTE", got)
	}
	if got := accessTechnology((1 << 15) | (1 << 14)); got != "5G NR / 4G LTE" {
		t.Fatalf("combined technology = %q, want 5G NR / 4G LTE", got)
	}
	if got := accessTechnology(0); got != "未知制式" {
		t.Fatalf("unknown technology = %q, want 未知制式", got)
	}
}
