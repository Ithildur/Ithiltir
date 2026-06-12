package metrics

import "testing"

func TestToSnapshotExtractsCPUTemperatureHistoryColumn(t *testing.T) {
	cpuTemp := 62.5
	otherCPU := 58.0

	input := Metrics{
		Thermal: &Thermal{
			Status: "ok",
			Sensors: []ThermalSensor{
				{Kind: "cpu", Name: "k10temp_tctl", SensorKey: "k10temp_tctl", Source: "gopsutil", Status: "ok", TempC: &cpuTemp},
				{Kind: "cpu", Name: "k10temp_tccd1", SensorKey: "k10temp_tccd1", Source: "gopsutil", Status: "ok", TempC: &otherCPU},
			},
		},
	}

	snapshot, err := ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if snapshot.CPUTempC == nil || *snapshot.CPUTempC != cpuTemp {
		t.Fatalf("CPUTempC = %v, want %.1f", snapshot.CPUTempC, cpuTemp)
	}
}

func TestSnapshotPreservesPressureNumericColumns(t *testing.T) {
	input := Metrics{
		Pressure: &Pressure{
			CPU: &PressureResource{
				Some: &PressureStats{Avg10: 10, Avg60: 20, Avg300: 30, Total: 400},
			},
			Memory: &PressureResource{
				Full: &PressureStats{Avg10: 1, Avg60: 2, Avg300: 3, Total: 4},
			},
			IO: &PressureResource{
				Some: &PressureStats{Avg10: 5, Avg60: 6, Avg300: 7, Total: 8},
				Full: &PressureStats{Avg10: 9, Avg60: 10, Avg300: 11, Total: 12},
			},
		},
	}

	snapshot, err := ToSnapshot(input)
	if err != nil {
		t.Fatalf("ToSnapshot() error = %v", err)
	}
	if snapshot.PSICPUSomeAvg300 == nil || *snapshot.PSICPUSomeAvg300 != 30 {
		t.Fatalf("PSICPUSomeAvg300 = %v, want 30", snapshot.PSICPUSomeAvg300)
	}
	if snapshot.PSIMemFullAvg300 == nil || *snapshot.PSIMemFullAvg300 != 3 {
		t.Fatalf("PSIMemFullAvg300 = %v, want 3", snapshot.PSIMemFullAvg300)
	}
	if snapshot.PSIIOFullTotal == nil || *snapshot.PSIIOFullTotal != 12 {
		t.Fatalf("PSIIOFullTotal = %v, want 12", snapshot.PSIIOFullTotal)
	}

	got, err := metricsFromSnapshot(snapshot)
	if err != nil {
		t.Fatalf("metricsFromSnapshot() error = %v", err)
	}
	if got.Pressure == nil || got.Pressure.CPU == nil || got.Pressure.CPU.Some == nil {
		t.Fatalf("pressure CPU some missing after snapshot roundtrip: %+v", got.Pressure)
	}
	if got.Pressure.CPU.Some.Avg300 != 30 {
		t.Fatalf("pressure CPU some avg300 = %v, want 30", got.Pressure.CPU.Some.Avg300)
	}
	if got.Pressure.Memory == nil || got.Pressure.Memory.Full == nil || got.Pressure.Memory.Full.Avg300 != 3 {
		t.Fatalf("pressure memory full = %+v, want avg300 3", got.Pressure.Memory)
	}
}
