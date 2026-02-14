import os
import sys
import json
import requests
from pathlib import Path

# ============================================================
# CONFIGURAÇÃO
# ============================================================
PROJECT_ID = "aurorav2-484411"
LOCATION = "us-central1"
# O usuário chamou de gemini-3-pro-preview, mas usaremos o ID estável de 2M se falhar
MODEL_ID = "gemini-1.5-pro-002" 
TOKEN = "AQ.Ab8RN6K0AGcknvGvIwblXQR6OY2LgzDH6v_iYBsqUPolUqaXMA"

# Diretórios para incluir na análise
INCLUDE_DIRS = ["internal", "pkg", "api", "cmd"]
INCLUDE_EXTENSIONS = [".go", ".mod", ".proto", ".md"]
EXCLUDE_DIRS = [".git", "vendor", "node_modules", "build", "dist", "tmp"]

def get_project_files(root_path):
    files_to_analyze = []
    root = Path(root_path)
    main_go = root / "main.go"
    if main_go.exists():
        files_to_analyze.append(main_go)
    for d in INCLUDE_DIRS:
        dir_path = root / d
        if dir_path.exists():
            for p in dir_path.rglob("*"):
                if p.is_file() and p.suffix in INCLUDE_EXTENSIONS:
                    if not any(ex in p.parts for ex in EXCLUDE_DIRS):
                        files_to_analyze.append(p)
    return files_to_analyze

def consolidate_code(files, root_path):
    consolidated = ""
    root = Path(root_path)
    total_bytes = 0
    for f in files:
        try:
            rel_path = f.relative_to(root)
            content = f.read_text(encoding='utf-8', errors='ignore')
            total_bytes += len(content)
            consolidated += f"\n\n// FILE: {rel_path}\n"
            consolidated += content
        except Exception as e:
            print(f"Erro ao ler {f}: {e}")
    return consolidated, total_bytes

def run_audit_rest(root_path):
    print(f"🚀 Coletando arquivos...")
    files = get_project_files(root_path)
    code, size = consolidate_code(files, root_path)
    print(f"📏 Tamanho: {size / 1024 / 1024:.2f} MB")

    url = f"https://{LOCATION}-aiplatform.googleapis.com/v1/projects/{PROJECT_ID}/locations/{LOCATION}/publishers/google/models/{MODEL_ID}:generateContent"
    
    headers = {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json"
    }

    payload = {
        "contents": [{
            "parts": [{
                "text": f"Realize uma auditoria PROFUNDA e COMPLETA de todo o código fonte fornecido abaixo: \n\n {code} \n\n Gere um relatório Markdown detalhado."
            }]
        }],
        "generationConfig": {
            "maxOutputTokens": 8192,
            "temperature": 0.2
        }
    }

    print(f"📤 Enviando requisição REST para Vertex AI ({MODEL_ID})...")
    response = requests.post(url, headers=headers, json=payload)
    
    if response.status_code == 200:
        data = response.json()
        try:
            report = data['candidates'][0]['content']['parts'][0]['text']
            Path("EVA_MIND_FULL_AUDIT.md").write_text(report, encoding="utf-8")
            print("✅ Auditoria concluída! Relatório salvo em: EVA_MIND_FULL_AUDIT.md")
        except Exception as e:
            print(f"❌ Erro ao processar resposta: {e}")
            print(json.dumps(data, indent=2))
    else:
        print(f"❌ Falha na API: {response.status_code}")
        print(response.text)

if __name__ == "__main__":
    run_audit_rest(".")
