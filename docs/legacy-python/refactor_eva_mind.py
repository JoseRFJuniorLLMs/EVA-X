import os
import sys
from pathlib import Path
import google.generativeai as genai

# ============================================================
# CONFIGURAÇÃO
# ============================================================
API_KEY = "AIzaSyBlem2g_EFVLTt3Fb1AofF1EOAf05YPo3U" 
MODEL_NAME = "gemini-3-pro-preview" # Modelo solicitado pelo usuário e disponível na lista

# Arquivos críticos para refatorar (baseado na auditoria)
CRITICAL_FILES = [
    "internal/voice/handler.go",
    "main.go",
    "internal/cortex/gemini/handler.go"
]

# ============================================================
# INICIALIZAÇÃO
# ============================================================
genai.configure(api_key=API_KEY)

def run_refactor(root_path):
    root = Path(root_path)
    audit_path = root / "EVA_MIND_FULL_AUDIT.md"
    
    if not audit_path.exists():
        print("❌ Relatório de auditoria não encontrado!")
        return

    audit_content = audit_path.read_text(encoding="utf-8")
    
    # Carregar o código completo para contexto (janela de 2M permite isso)
    # Mas vamos focar o output nos arquivos críticos
    all_go_files = list(root.rglob("*.go"))
    full_context = ""
    for f in all_go_files:
        if "vendor" in str(f) or ".git" in str(f): continue
        try:
            full_context += f"\n// FILE: {f.relative_to(root)}\n{f.read_text(encoding='utf-8', errors='ignore')}\n"
        except: continue

    model = genai.GenerativeModel(MODEL_NAME)

    prompt = f"""
Você é um Engenheiro de Software Sênior e Auditor. Recebeu esta AUDITORIA de um sistema em Go (EVA-Mind).

AUDITORIA:
{audit_content}

Sua tarefa é REESCREVER e FORNECER O CÓDIGO COMPLETO E PRONTO para os arquivos que precisam de correções críticas e quick wins.

Foque nos seguintes arquivos e aplique as melhorias sugeridas (DSP, Latência, Race Conditions, Refatoração do SignalingServer, Turn ID versioning):
1. internal/voice/handler.go
2. main.go
3. internal/cortex/gemini/handler.go

Para cada arquivo, forneça o código completo dentro de um bloco de código markdown, precedido pelo nome do arquivo.

O código deve estar pronto para compilar (Ready to use).
"""

    print(f"📤 Enviando auditoria e contexto ({len(full_context)/1024/1024:.2f} MB) para refatoração...")
    try:
        # Enviamos o código completo como contexto para que ele saiba como as peças se encaixam
        response = model.generate_content([prompt, full_context])
        refactored_result = response.text
        
        output_path = root / "EVA_MIND_REFACTORED_CODE.md"
        output_path.write_text(refactored_result, encoding="utf-8")
        print(f"✅ Versões refatoradas salvas em: {output_path}")
        
    except Exception as e:
        print(f"❌ Erro na chamada do Gemini: {e}")

if __name__ == "__main__":
    run_refactor(".")
