package libmpeg

import (
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

func TestTS_DeMux(t *testing.T) {
	type fields struct {
		SyncByte                   byte
		TransportErrorIndicator    uint8
		PayloadUnitStartIndicator  uint8
		TransportPriority          uint8
		PID                        uint16
		TransportScramblingControl byte
		AdaptationFieldExist       byte
		ContinuityCounter          byte
		AdaptationField            *AdaptationField
		PayloadPointer             uint8
		Payload                    PSI
	}
	type args struct {
		pidTable map[uint16]PSI
		reader   easyio.EasyReader
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TS{
				SyncByte:                   tt.fields.SyncByte,
				TransportErrorIndicator:    tt.fields.TransportErrorIndicator,
				PayloadUnitStartIndicator:  tt.fields.PayloadUnitStartIndicator,
				TransportPriority:          tt.fields.TransportPriority,
				PID:                        tt.fields.PID,
				TransportScramblingControl: tt.fields.TransportScramblingControl,
				AdaptationFieldExist:       tt.fields.AdaptationFieldExist,
				ContinuityCounter:          tt.fields.ContinuityCounter,
				AdaptationField:            tt.fields.AdaptationField,
				PayloadPointer:             tt.fields.PayloadPointer,
				Payload:                    tt.fields.Payload,
			}
			if err := ts.DeMux(tt.args.pidTable, tt.args.reader); (err != nil) != tt.wantErr {
				t.Errorf("TS.DeMux() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTS_Mux(t *testing.T) {
	type fields struct {
		SyncByte                   byte
		TransportErrorIndicator    uint8
		PayloadUnitStartIndicator  uint8
		TransportPriority          uint8
		PID                        uint16
		TransportScramblingControl byte
		AdaptationFieldExist       byte
		ContinuityCounter          byte
		AdaptationField            *AdaptationField
		PayloadPointer             uint8
		Payload                    PSI
	}
	type args struct {
		pidTable map[uint16]PSI
		writer   easyio.EasyWriter
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TS{
				SyncByte:                   tt.fields.SyncByte,
				TransportErrorIndicator:    tt.fields.TransportErrorIndicator,
				PayloadUnitStartIndicator:  tt.fields.PayloadUnitStartIndicator,
				TransportPriority:          tt.fields.TransportPriority,
				PID:                        tt.fields.PID,
				TransportScramblingControl: tt.fields.TransportScramblingControl,
				AdaptationFieldExist:       tt.fields.AdaptationFieldExist,
				ContinuityCounter:          tt.fields.ContinuityCounter,
				AdaptationField:            tt.fields.AdaptationField,
				PayloadPointer:             tt.fields.PayloadPointer,
				Payload:                    tt.fields.Payload,
			}
			if err := ts.Mux(tt.args.pidTable, tt.args.writer); (err != nil) != tt.wantErr {
				t.Errorf("TS.Mux() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
