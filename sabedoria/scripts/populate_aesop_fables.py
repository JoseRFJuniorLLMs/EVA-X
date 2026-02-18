#!/usr/bin/env python3
"""
Fábulas de Esopo → Qdrant com Schema Lacaniano
Ponte Semântica: TransNAR (Lacan) ↔ Esopo (Moral/Estrutura)

Collection: aesop_fables (Separada de nasrudin_stories)
Zeta Affinity: Tipos Racionais (1, 3, 5, 6)

Uso no servidor:
    python3 populate_aesop_fables.py
"""

import requests
import json
import re
import time
from typing import List, Dict, Optional

# ============================================================================
# CONFIGURAÇÕES
# ============================================================================
QDRANT_URL = "http://localhost:6333"
OLLAMA_URL = "http://localhost:11434"
COLLECTION_NAME = "aesop_fables"
BOOK_PATH = "/root/EVA-Mind-FZPN/docs/Fabulas_Isopo.txt"

# ============================================================================
# MAPEAMENTO LACANIANO MANUAL (Fábulas-Chave)
# ============================================================================
LACANIAN_MAPPING = {
    "XLVIII": {  # A Raposa e as Uvas
        "title": "A Raposa e as Uvas",
        "transnar_rule": ["projection", "rationalization"],
        "lacanian_concept": "sour_grapes_mechanism",
        "defense_mechanism": "rationalization",
        "emotional_state": ["frustration", "denial", "disdain"],
        "zeta_affinity": [1, 3, 5, 6],  # Racionais
        "trigger_condition": "User dismisses goals or people after failing to achieve/obtain them",
        "eva_followup": "Isso me lembra a Raposa de Esopo. Ela disse que as uvas estavam verdes só porque não as alcançava. Será que estamos desdenhando desse objetivo só porque ele ficou difícil?"
    },
    "I": {  # O Galo e a Pérola
        "title": "O Galo e a Pérola",
        "transnar_rule": ["sublimation_failure"],
        "lacanian_concept": "symbolic_value_blindness",
        "defense_mechanism": "intellectualization",
        "emotional_state": ["superficiality", "materialism"],
        "zeta_affinity": [3, 7],
        "trigger_condition": "User dismisses valuable advice in favor of immediate trivial comfort",
        "eva_followup": "Às vezes somos como o galo que prefere a migalha e chuta a pérola da sabedoria. Será que estamos valorizando o que realmente importa?"
    },
    "XXI": {  # O Menino que Gritava Lobo (se existir no arquivo)
        "title": "O Pastor e o Lobo",
        "transnar_rule": ["hysteria", "attention_seeking"],
        "lacanian_concept": "symbolic_law_breach",
        "defense_mechanism": "dramatization",
        "emotional_state": ["attention_seeking", "manipulation"],
        "zeta_affinity": [2, 3, 6, 7],
        "trigger_condition": "User repeatedly raises false alarms or exaggerates symptoms for attention",
        "eva_followup": "É perigoso brincar com coisas sérias. Lembra da história do menino e o lobo? A confiança é um cristal difícil de colar."
    },
    "XXXII": {  # O Cão e o Osso (se for sobre inveja)
        "title": "O Cão e sua Sombra",
        "transnar_rule": ["envy", "dissatisfaction"],
        "lacanian_concept": "mirror_stage_failure",
        "defense_mechanism": "projection",
        "emotional_state": ["greed", "dissatisfaction", "envy"],
        "zeta_affinity": [1, 3, 6],
        "trigger_condition": "User constantly compares self to others and feels dissatisfied",
        "eva_followup": "Como o cão que largou seu osso para pegar o reflexo na água. Às vezes perdemos o que temos ao cobiçar o que parece melhor."
    }
}

# ============================================================================
# FUNÇÕES AUXILIARES
# ============================================================================

def print_progress(current, total, prefix='', suffix=''):
    """Barra de progresso visual"""
    percent = int(100 * current / total)
    filled = int(40 * current / total)
    bar = '█' * filled + '-' * (40 - filled)
    print(f'\r{prefix} |{bar}| {percent}% {suffix}', end='\r')
    if current == total:
        print()

def generate_embedding_ollama(text: str) -> Optional[List[float]]:
    """Gera embedding usando Ollama (nomic-embed-text)"""
    try:
        response = requests.post(
            f"{OLLAMA_URL}/api/embeddings",
            json={
                "model": "nomic-embed-text",
                "prompt": text[:2000]
            },
            timeout=30
        )
        if response.status_code == 200:
            return response.json()["embedding"]
        return None
    except Exception as e:
        print(f"\n❌ Erro embedding: {e}")
        return None

def parse_fables(file_path: str) -> List[Dict]:
    """Parse do arquivo Fabulas_Isopo.txt"""
    print("\n📖 Lendo Fábulas de Esopo...")
    
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    fables = []
    
    # Pattern: "Fábula [ROMANO]\n[Título]\n[Texto]...Moral da história:"
    pattern = r'Fábula ([IVXLCDM]+)\n(.*?)\n(.*?)(?=Fábula [IVXLCDM]+|\Z)'
    matches = re.findall(pattern, content, re.DOTALL)
    
    print(f"✅ Encontradas {len(matches)} fábulas\n")
    
    for roman_num, title, full_text in matches:
        # Separar narrativa e moral
        if "Moral da história" in full_text:
            parts = full_text.split("Moral da história")
            narrative = parts[0].strip()
            moral = parts[1].strip() if len(parts) > 1 else ""
        else:
            narrative = full_text.strip()
            moral = ""
        
        # Pular se muito curto
        if len(narrative) < 50:
            continue
        
        # Criar payload base
        payload = {
            "fable_id": f"aesop_{roman_num.lower()}",
            "roman_number": roman_num,
            "title": title.strip(),
            "text_narrative": narrative,
            "text_moral": moral,
            "language": "pt-BR",
            "source": "Fabulas de Esopo - Carlos Pinheiro"
        }
        
        # Adicionar mapeamento Lacaniano se existir
        if roman_num in LACANIAN_MAPPING:
            mapping = LACANIAN_MAPPING[roman_num]
            payload["clinical_tags"] = {
                "transnar_rule": mapping["transnar_rule"],
                "lacanian_concept": mapping["lacanian_concept"],
                "defense_mechanism": mapping["defense_mechanism"],
                "emotional_state": mapping["emotional_state"],
                "zeta_affinity": mapping["zeta_affinity"]
            }
            payload["trigger_condition"] = mapping["trigger_condition"]
            payload["eva_followup"] = mapping["eva_followup"]
            payload["is_clinically_mapped"] = True
        else:
            # Todas as fábulas de Esopo têm afinidade com tipos racionais
            payload["clinical_tags"] = {
                "zeta_affinity": [1, 3, 5, 6]  # Default: Racionais
            }
            payload["is_clinically_mapped"] = False
        
        fables.append(payload)
    
    return fables

def create_collection():
    """Cria collection no Qdrant"""
    print("\n🔧 Configurando Qdrant (Collection: aesop_fables)...")
    
    # Deletar se existir
    response = requests.get(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
    if response.status_code == 200:
        print(f"⚠️  Deletando collection existente...")
        requests.delete(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
        time.sleep(1)
    
    # Criar nova
    payload = {
        "vectors": {
            "size": 768,
            "distance": "Cosine"
        },
        "on_disk_payload": True
    }
    
    response = requests.put(
        f"{QDRANT_URL}/collections/{COLLECTION_NAME}",
        json=payload
    )
    
    if response.status_code == 200:
        print(f"✅ Collection '{COLLECTION_NAME}' criada\n")
    else:
        print(f"❌ Erro: {response.text}")
        exit(1)

def insert_fable(fable: Dict, point_id: int) -> bool:
    """Insere fábula no Qdrant com embedding"""
    
    # Gerar texto para embedding
    if fable.get("is_clinically_mapped"):
        embed_text = f"{fable['trigger_condition']}. {fable['title']}. {fable['text_narrative'][:500]}. Moral: {fable['text_moral'][:200]}"
    else:
        embed_text = f"{fable['title']}. {fable['text_narrative'][:500]}. Moral: {fable['text_moral'][:200]}"
    
    # Gerar embedding
    vector = generate_embedding_ollama(embed_text)
    if vector is None:
        return False
    
    # Criar ponto
    point = {
        "id": point_id,
        "vector": vector,
        "payload": fable
    }
    
    # Inserir
    response = requests.put(
        f"{QDRANT_URL}/collections/{COLLECTION_NAME}/points",
        json={"points": [point]}
    )
    
    return response.status_code == 200

# ============================================================================
# MAIN
# ============================================================================

def main():
    print("=" * 70)
    print("📚 ESOPO (MORAL/ESTRUTURA) → QDRANT")
    print("   Zeta Affinity: Tipos Racionais (1, 3, 5, 6)")
    print("=" * 70)
    
    # 1. Parse
    fables = parse_fables(BOOK_PATH)
    total = len(fables)
    
    # Contar mapeadas
    mapped = sum(1 for f in fables if f.get("is_clinically_mapped"))
    print(f"📊 Total: {total} fábulas ({mapped} com mapeamento Lacaniano)\n")
    
    # 2. Criar collection
    create_collection()
    
    # 3. Inserir
    print(f"📥 Inserindo no Qdrant...\n")
    
    success = 0
    failed = 0
    
    for idx, fable in enumerate(fables, 1):
        print_progress(
            idx, total,
            prefix=f'Progresso ({idx}/{total}):',
            suffix=f'✅ {success} | ❌ {failed}'
        )
        
        if insert_fable(fable, idx):
            success += 1
        else:
            failed += 1
        
        time.sleep(0.3)
    
    # 4. Verificar
    print("\n" + "=" * 70)
    response = requests.get(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
    
    if response.status_code == 200:
        data = response.json()
        print(f"\n📊 RESULTADO:")
        print(f"   ✅ Inseridas: {success}")
        print(f"   ❌ Falhas: {failed}")
        print(f"   🧠 Com Schema Lacaniano: {mapped}")
        print(f"   📦 Points no Qdrant: {data['result']['points_count']}")
        print("\n✨ Esopo (Moral/Estrutura) estabelecido!")
        print("   → Para Tipos Racionais: 1, 3, 5, 6")
    
    print("=" * 70)

if __name__ == "__main__":
    main()
