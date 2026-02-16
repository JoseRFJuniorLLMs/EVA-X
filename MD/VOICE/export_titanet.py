#!/usr/bin/env python3
"""
export_titanet.py — Exporta o TitaNet-Large para ONNX (rodar UMA VEZ).

Requisitos:
    pip install nemo_toolkit[asr] onnx onnxsim torch

Uso:
    python3 export_titanet.py
    scp titanet_large.onnx root@104.248.219.200:/opt/eva/models/

O arquivo gerado é usado pelo eva-mind via ONNX Runtime Go.
Não é necessário Python no servidor de produção após este passo.
"""

import torch
import nemo.collections.asr as nemo_asr
import onnx
from onnxsim import simplify

OUTPUT_PATH = "titanet_large.onnx"

print("[1/4] Carregando TitaNet-Large...")
model = nemo_asr.models.EncDecSpeakerLabelModel.from_pretrained(
    "nvidia/speakerverification_en_titanet_large"
)
model.eval()
model = model.cpu()

print("[2/4] Exportando para ONNX...")
# Input: (batch=1, n_mels=80, n_frames) — dinâmico no eixo de frames
dummy_audio = torch.randn(1, 80, 200)   # 200 frames ≈ 2s
dummy_length = torch.tensor([200], dtype=torch.long)

torch.onnx.export(
    model,
    (dummy_audio, dummy_length),
    OUTPUT_PATH,
    input_names=["audio_signal", "length"],
    output_names=["logits", "embs"],
    dynamic_axes={
        "audio_signal": {0: "batch", 2: "frames"},
        "length":       {0: "batch"},
        "logits":       {0: "batch"},
        "embs":         {0: "batch"},
    },
    opset_version=17,
    export_params=True,
    do_constant_folding=True,
)

print("[3/4] Simplificando grafo ONNX...")
model_onnx = onnx.load(OUTPUT_PATH)
model_simplified, check = simplify(model_onnx)
if check:
    onnx.save(model_simplified, OUTPUT_PATH)
    print("    Simplificação OK")
else:
    print("    Simplificação falhou — usando original")

print("[4/4] Validando...")
onnx.checker.check_model(OUTPUT_PATH)
print(f"\n✅ Modelo exportado com sucesso: {OUTPUT_PATH}")
print(f"   Tamanho: {__import__('os').path.getsize(OUTPUT_PATH) / 1e6:.1f} MB")
print(f"\nPróximo passo:")
print(f"   scp {OUTPUT_PATH} root@104.248.219.200:/opt/eva/models/")
