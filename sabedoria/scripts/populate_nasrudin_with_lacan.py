#!/usr/bin/env python3
"""
Nasrudin Stories → Qdrant com Schema Lacaniano
Ponte Semântica: TransNAR (Lacan) ↔ Nasrudin (Paradoxo)

Uso no servidor:
    python3 populate_nasrudin_with_lacan.py
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
COLLECTION_NAME = "nasrudin_stories"
BOOK_PATH = "/root/EVA-Mind-FZPN/docs/book1.txt"

# ============================================================================
# MAPEAMENTO LACANIANO MANUAL (Histórias-Chave)
# ============================================================================
LACANIAN_MAPPING = {
    "215": {  # A Chave e a Luz
        "title": "A Chave e a Luz",
        "transnar_rule": "negation_as_desire",
        "lacanian_concept": "objet_petit_a",
        "defense_mechanism": "displacement",
        "emotional_state": ["avoidance", "convenience", "superficiality"],
        "trigger_condition": "User avoids looking at the real problem because it's painful",
        "eva_followup": "Às vezes procuramos a solução onde é mais confortável, não onde a dor realmente está. Será que estamos procurando a chave na luz?"
    },
    "250": {  # A Nota Única
        "title": "A Nota Única",
        "transnar_rule": "compulsive_repetition",
        "lacanian_concept": "the_real",
        "defense_mechanism": "fixation",
        "emotional_state": ["stubbornness", "monotony", "rigidity"],
        "trigger_condition": "User repeats the same complaint without changing perspective",
        "eva_followup": "Você me lembra o Nasrudin tocando alaúde. Talvez sinta que já 'encontrou' sua verdade e parou de procurar outras melodias?"
    },
    "208": {  # O Burro ao Contrário
        "title": "O Burro ao Contrário",
        "transnar_rule": "projection",
        "lacanian_concept": "mirror_stage",
        "defense_mechanism": "externalization",
        "emotional_state": ["blame", "lack_of_accountability"],
        "trigger_condition": "User blames external factors for their own choices",
        "eva_followup": "É tentador pensar que foi o mundo que virou ao contrário, não é? Mas quem está segurando as rédeas?"
    },
    "206": {  # O Gato e a Carne
        "title": "O Gato e a Carne",
        "transnar_rule": "internal_contradiction",
        "lacanian_concept": "symbolic_order",
        "defense_mechanism": "rationalization",
        "emotional_state": ["confusion", "cognitive_dissonance"],
        "trigger_condition": "User presents contradictory statements about same situation",
        "eva_followup": "Se isto é o gato, onde está a carne? Às vezes a verdade se esconde nas contradições."
    },
    "233": {  # A Lua no Poço
        "title": "A Lua no Poço",
        "transnar_rule": "reactive_formation",
        "lacanian_concept": "imaginary",
        "defense_mechanism": "magical_thinking",
        "emotional_state": ["grandiosity", "false_achievement"],
        "trigger_condition": "User takes credit for changes that happened naturally",
        "eva_followup": "Às vezes nos damos crédito por mudanças que aconteceriam de qualquer forma, como a lua voltando ao céu."
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
                "prompt": text[:2000]  # Limitar tamanho
            },
            timeout=30
        )
        if response.status_code == 200:
            return response.json()["embedding"]
        return None
    except Exception as e:
        print(f"\n❌ Erro embedding: {e}")
        return None

def parse_stories(file_path: str) -> List[Dict]:
    """Parse do arquivo book1.txt"""
    print("\n📖 Lendo histórias de Nasrudin...")
    
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    stories = []
    pattern = r'(\d+)\.\n(.*?)(?=\n\d+\.\n|\nCAPÍTULO|\Z)'
    matches = re.findall(pattern, content, re.DOTALL)
    
    print(f"✅ Encontradas {len(matches)} histórias\n")
    
    for story_num, story_text in matches:
        story_text = story_text.strip()
        if len(story_text) < 50:
            continue
        
        # Extrair título
        lines = story_text.split('\n')
        title = f"História {story_num}"
        for line in lines[:3]:
            if line.isupper() or '"' in line:
                title = line.strip('"').strip()
                break
        
        # Criar payload base
        payload = {
            "story_id": f"nasrudin_{story_num.zfill(3)}",
            "story_number": int(story_num),
            "title": title,
            "text": story_text,
            "language": "pt-BR",
            "source": "book1.txt"
        }
        
        # Adicionar mapeamento Lacaniano se existir
        if story_num in LACANIAN_MAPPING:
            mapping = LACANIAN_MAPPING[story_num]
            payload["clinical_tags"] = {
                "transnar_rule": mapping["transnar_rule"],
                "lacanian_concept": mapping["lacanian_concept"],
                "defense_mechanism": mapping["defense_mechanism"],
                "emotional_state": mapping["emotional_state"]
            }
            payload["trigger_condition"] = mapping["trigger_condition"]
            payload["eva_followup"] = mapping["eva_followup"]
            payload["is_clinically_mapped"] = True
        else:
            payload["is_clinically_mapped"] = False
        
        stories.append(payload)
    
    return stories

def create_collection():
    """Cria collection no Qdrant"""
    print("\n🔧 Configurando Qdrant...")
    
    # Deletar se existir
    response = requests.get(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
    if response.status_code == 200:
        print(f"⚠️  Deletando collection existente...")
        requests.delete(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
        time.sleep(1)
    
    # Criar nova (768 dimensões = nomic-embed-text)
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

def insert_story(story: Dict, point_id: int) -> bool:
    """Insere história no Qdrant com embedding"""
    
    # Gerar texto para embedding
    # Se tem mapeamento Lacaniano, incluir trigger_condition
    if story.get("is_clinically_mapped"):
        embed_text = f"{story['trigger_condition']}. {story['title']}. {story['text'][:500]}"
    else:
        embed_text = f"{story['title']}. {story['text'][:500]}"
    
    # Gerar embedding
    vector = generate_embedding_ollama(embed_text)
    if vector is None:
        return False
    
    # Criar ponto
    point = {
        "id": point_id,
        "vector": vector,
        "payload": story
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
    print("🧠 PONTE LACAN-NASRUDIN → QDRANT")
    print("=" * 70)
    
    # 1. Parse
    stories = parse_stories(BOOK_PATH)
    total = len(stories)
    
    # Contar mapeadas
    mapped = sum(1 for s in stories if s.get("is_clinically_mapped"))
    print(f"📊 Total: {total} histórias ({mapped} com mapeamento Lacaniano)\n")
    
    # 2. Criar collection
    create_collection()
    
    # 3. Inserir
    print(f"📥 Inserindo no Qdrant...\n")
    
    success = 0
    failed = 0
    
    for idx, story in enumerate(stories, 1):
        print_progress(
            idx, total,
            prefix=f'Progresso ({idx}/{total}):',
            suffix=f'✅ {success} | ❌ {failed}'
        )
        
        if insert_story(story, idx):
            success += 1
        else:
            failed += 1
        
        time.sleep(0.3)  # Rate limiting
    
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
        print("\n✨ Ponte Lacan-Nasrudin estabelecida!")
    
    print("=" * 70)

if __name__ == "__main__":
    main()
