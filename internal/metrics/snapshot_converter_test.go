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
