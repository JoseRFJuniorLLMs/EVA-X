import os
import sys
from pathlib import Path
import google.generativeai as genai

# ============================================================
# CONFIGURAÇÃO
# ============================================================
API_KEY = "AIzaSyBlem2g_EFVLTt3Fb1AofF1EOAf05YPo3U" 
MODEL_NAME = "gemini-3-pro-preview" # Como solicitado explicitamente pelo usuário

# Diretórios para incluir na análise
INCLUDE_DIRS = ["internal", "pkg", "api", "cmd"]
INCLUDE_EXTENSIONS = [".go", ".mod", ".proto", ".md"]
EXCLUDE_DIRS = [".git", "vendor", "node_modules", "build", "dist", "tmp"]

# ============================================================
# INICIALIZAÇÃO
# ============================================================
genai.configure(api_key=API_KEY)

def get_project_files(root_path):
    """Coleta todos os arquivos relevantes do projeto."""
    files_to_analyze = []
    root = Path(root_path)
    
    # Adicionar main.go se existir
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
    """Consolida o código de todos os arquivos em uma string formatada."""
    consolidated = ""
    root = Path(root_path)
    total_bytes = 0
    
    for f in files:
        try:
            rel_path = f.relative_to(root)
            content = f.read_text(encoding='utf-8', errors='ignore')
            total_bytes += len(content)
            
            consolidated += f"\n\n// ============================================================\n"
            consolidated += f"// FILE: {rel_path}\n"
            consolidated += f"// ============================================================\n\n"
            consolidated += content
            consolidated += "\n"
        except Exception as e:
            print(f"Erro ao ler {f}: {e}")
            
    return consolidated, total_bytes

def run_audit(root_path, dry_run=False):
    """Executa a auditoria completa."""
    print(f"🚀 Iniciando coleta de arquivos em: {root_path}")
    files = get_project_files(root_path)
    print(f"📊 Encontrados {len(files)} arquivos relevantes.")
    
    code, size = consolidate_code(files, root_path)
    print(f"📏 Tamanho total do código: {size / 1024 / 1024:.2f} MB")
    
    if dry_run:
        output_path = "EVA_MIND_CONSOLIDATED.txt"
        with open(output_path, "w", encoding="utf-8") as f:
            f.write(code)
        print(f"✅ [DRY RUN] Código consolidado salvo em: {output_path}")
        return

    print(f"🧠 Inicializando modelo {MODEL_NAME}...")
    model = genai.GenerativeModel(MODEL_NAME)

    prompt = f"""
Você é um auditor sênior de sistemas distribuídos e IA conversacional. Sua missão é realizar uma Auditoria de Projeto Completa do sistema EVA-Mind.

CONTEXTO DO PROJETO:
O EVA-Mind é um sistema de voz terapêutico para idosos baseado em psicanálise Lacaniana, Eneagrama e integração com Google Gemini Live API. 
O sistema utiliza Go no backend, com integrações complexas de áudio (PCM), WebSockets e pipelines de memória (Neo4j/Qdrant).

TAREFA:
Realize uma auditoria PROFUNDA e COMPLETA de todo o código fonte fornecido abaixo. Baseie sua análise nos seguintes pilares:

1. **Arquitetura & Design**: O sistema é escalável? O desacoplamento entre o 'Cortex' (lógica de IA) e os handlers de IO está correto?
2. **Concorrência em Go**: Procure por race conditions, deadlocks em mutexes, e vazamento de goroutines. O uso de contextos está onipresente?
3. **Pipeline de Áudio & Latência**: Verifique a eficiência no processamento de buffers PCM e se há gargalos que podem afetar o tempo de resposta da voz.
4. **Segurança & Resiliência**: Gestão de segredos, validação de inputs e estratégias de retry em APIs externas.
5. **Fidelidade Clínica (Lacano/Eneagrama)**: Verifique se a implementação técnica reflete corretamente os conceitos de FDPN (Grafo do Desejo) e RSI.
6. **Memória Episódica e Semântica**: Analise a lógica de compressão de fatos atômicos e busca vetorial (Qdrant).

CÓDIGO FONTE COMPLETO:
{code}

FORMATO DA RESPOSTA (Markdown):
# RELATÓRIO DE AUDITORIA COMPLETA - EVA-MIND
- **Resumo Executivo** (Estado Geral do Sistema)
- **Score de Qualidade** (0-100)
- **Identificação de Ganhos Rápidos** (Quick Wins)
- 🚨 **RISCOS CRÍTICOS** (Ações imediatas necessárias)
- 🔍 **ANÁLISE DETALHADA POR MÓDULO**
- 📈 **RECOMENDAÇÕES ESTRATÉGICAS DE LONGO PRAZO**
"""

    print("📤 Enviando para o Gemini... Isso pode levar alguns minutos devido ao volume de dados.")
    try:
        response = model.generate_content(prompt)
        report = response.text
        
        report_path = "EVA_MIND_FULL_AUDIT.md"
        with open(report_path, "w", encoding="utf-8") as f:
            f.write(report)
        print(f"✅ Auditoria concluída! Relatório salvo em: {report_path}")
    except Exception as e:
        print(f"❌ Erro na chamada do Gemini: {e}")

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true", help="Gera apenas o arquivo consolidado localmente")
    parser.add_argument("--path", default=".", help="Caminho raiz do projeto")
    args = parser.parse_args()
    
    run_audit(args.path, dry_run=args.dry_run)
