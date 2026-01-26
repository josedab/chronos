package cron

import (
	"testing"
	"time"
)

func BenchmarkParser_Parse_Standard(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Parse("*/5 * * * *")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser_Parse_Extended(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Parse("30 9 * * 1-5")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser_Parse_Descriptor(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Parse("@every 30m")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSchedule_Next(b *testing.B) {
	schedule, err := Parse("*/5 * * * *")
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		schedule.Next(now)
	}
}

func BenchmarkSchedule_Next_Complex(b *testing.B) {
	schedule, err := Parse("0 9-17 * * 1-5")
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		schedule.Next(now)
	}
}
