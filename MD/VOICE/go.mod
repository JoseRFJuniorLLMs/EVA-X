module github.com/eva-project/eva-mind/voice

go 1.22

require (
	// ONNX Runtime Go bindings — para rodar TitaNet.onnx nativamente
	github.com/yalue/onnxruntime_go v1.13.0

	// Neo4j driver oficial
	github.com/neo4j/neo4j-go-driver/v5 v5.20.0

	// Logger estruturado
	go.uber.org/zap v1.27.0
)

// ─── Instruções de Setup ──────────────────────────────────────────────────
//
// 1. ONNX Runtime (instalar no servidor):
//    wget https://github.com/microsoft/onnxruntime/releases/download/v1.17.3/onnxruntime-linux-x64-1.17.3.tgz
//    tar -xzf onnxruntime-linux-x64-1.17.3.tgz
//    cp onnxruntime-linux-x64-1.17.3/lib/libonnxruntime.so.1.17.3 /usr/local/lib/libonnxruntime.so
//    ldconfig
//
// 2. Exportar TitaNet para ONNX (rodar UMA VEZ, em qualquer máquina com GPU/CPU):
//    pip install nemo_toolkit[asr] onnx onnxsim
//    python3 export_titanet.py  (veja scripts/export_titanet.py)
//    scp titanet_large.onnx root@104.248.219.200:/opt/eva/models/
//
// 3. Build do eva-mind:
//    go mod tidy
//    CGO_ENABLED=1 go build ./...
