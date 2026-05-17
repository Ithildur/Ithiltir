package metrics

func normalizeDiskSmart(in *DiskSmart) *DiskSmart {
	if in == nil {
		return nil
	}
	out := *in
	if out.Devices == nil {
		out.Devices = []DiskSmartDevice{}
	} else {
		out.Devices = make([]DiskSmartDevice, 0, len(in.Devices))
		for _, device := range in.Devices {
			if invalidTempC(device.TempC) {
				device.TempC = nil
			}
			out.Devices = append(out.Devices, device)
		}
	}
	if out.UpdatedAt != nil {
		utc := out.UpdatedAt.UTC()
		out.UpdatedAt = &utc
	}
	return &out
}

func normalizeThermal(in *Thermal) *Thermal {
	if in == nil {
		return nil
	}
	out := *in
	if out.Sensors == nil {
		out.Sensors = []ThermalSensor{}
	} else {
		out.Sensors = make([]ThermalSensor, 0, len(in.Sensors))
		for _, sensor := range in.Sensors {
			if invalidTempC(sensor.TempC) {
				sensor.TempC = nil
			}
			if invalidTempC(sensor.HighC) {
				sensor.HighC = nil
			}
			if invalidTempC(sensor.CriticalC) {
				sensor.CriticalC = nil
			}
			if sensor.TempC == nil {
				continue
			}
			out.Sensors = append(out.Sensors, sensor)
		}
	}
	if out.UpdatedAt != nil {
		utc := out.UpdatedAt.UTC()
		out.UpdatedAt = &utc
	}
	return &out
}

func invalidTempC(v *float64) bool {
	return v != nil && *v <= 0
}

func validTempC(v *float64) bool {
	return v != nil && *v > 0
}
