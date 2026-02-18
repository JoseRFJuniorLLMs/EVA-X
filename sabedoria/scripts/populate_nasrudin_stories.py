#!/usr/bin/env python3
"""
Script para popular histórias de Nasrudin no Qdrant
Lê o arquivo book1.txt, gera embeddings e insere no Qdrant
"""

import requests
import json
import re
import time
from typing import List, Dict
from openai import OpenAI

# Configurações
QDRANT_URL = "http://104.248.219.200:6333"  # Servidor remoto
GEMINI_API_KEY = "AIzaSyBnSKHtKNKJVxO-qvABiWPPJVWJzLlOhYo"
COLLECTION_NAME = "nasrudin_stories"
BOOK_PATH = "d:/dev/EVA/EVA-Mind-FZPN/docs/book1.txt"

# Cliente OpenAI (compatível com Gemini)
client = OpenAI(
    api_key=GEMINI_API_KEY,
    base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
)

def print_progress_bar(iteration, total, prefix='', suffix='', length=50, fill='█'):
    """Imprime barra de progresso visual"""
    percent = ("{0:.1f}").format(100 * (iteration / float(total)))
    filled_length = int(length * iteration // total)
    bar = fill * filled_length + '-' * (length - filled_length)
    print(f'\r{prefix} |{bar}| {percent}% {suffix}', end='\r')
    if iteration == total:
        print()

def parse_stories(file_path: str) -> List[Dict]:
    """Parse do arquivo de histórias"""
    print("\n📖 Lendo arquivo de histórias...")
    
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Dividir por números de história (formato: "203.\n")
    stories = []
    pattern = r'(\d+)\.\n(.*?)(?=\n\d+\.\n|\nCAPÍTULO|\Z)'
    matches = re.findall(pattern, content, re.DOTALL)
    
    print(f"✅ Encontradas {len(matches)} histórias\n")
    
    for story_num, story_text in matches:
        # Limpar texto
        story_text = story_text.strip()
        
        # Pular se for muito curto (provavelmente não é uma história)
        if len(story_text) < 50:
            continue
        
        # Extrair título (geralmente a primeira linha em maiúsculas ou entre aspas)
        lines = story_text.split('\n')
        title = f"História {story_num}"
        
        # Tentar encontrar título melhor
        for line in lines[:3]:
            if line.isupper() or '"' in line:
                title = line.strip('"').strip()
                break
        
        stories.append({
            "story_id": f"nasrudin_{story_num.zfill(3)}",
            "story_number": int(story_num),
            "title": title,
            "text": story_text,
            "language": "pt-BR",
            "source": "book1.txt"
        })
    
    return stories

def generate_embedding(text: str, retry_count=3) -> List[float]:
    """Gera embedding usando Gemini"""
    for attempt in range(retry_count):
        try:
            response = client.embeddings.create(
                model="text-embedding-004",
                input=text[:8000]  # Limitar tamanho
            )
            return response.data[0].embedding
        except Exception as e:
            if attempt < retry_count - 1:
                time.sleep(2 ** attempt)  # Backoff exponencial
                continue
            else:
                print(f"\n❌ Erro ao gerar embedding: {e}")
                return None

def create_collection():
    """Cria collection no Qdrant"""
    print("\n🔧 Criando collection no Qdrant...")
    
    # Verificar se já existe
    response = requests.get(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
    
    if response.status_code == 200:
        print(f"⚠️  Collection '{COLLECTION_NAME}' já existe. Deletando...")
        requests.delete(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
        time.sleep(1)
    
    # Criar nova collection
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
        print(f"✅ Collection '{COLLECTION_NAME}' criada com sucesso\n")
    else:
        print(f"❌ Erro ao criar collection: {response.text}")
        exit(1)

def insert_story(story: Dict, point_id: int) -> bool:
    """Insere uma história no Qdrant"""
    # Gerar embedding
    full_text = f"{story['title']}. {story['text']}"
    vector = generate_embedding(full_text)
    
    if vector is None:
        return False
    
    # Estrutura do ponto
    point = {
        "id": point_id,
        "vector": vector,
        "payload": story
    }
    
    # Inserir no Qdrant
    response = requests.put(
        f"{QDRANT_URL}/collections/{COLLECTION_NAME}/points",
        json={"points": [point]}
    )
    
    return response.status_code == 200

def main():
    print("=" * 70)
    print("🚀 NASRUDIN STORIES → QDRANT")
    print("=" * 70)
    
    # 1. Parse das histórias
    stories = parse_stories(BOOK_PATH)
    total = len(stories)
    
    if total == 0:
        print("❌ Nenhuma história encontrada!")
        return
    
    # 2. Criar collection
    create_collection()
    
    # 3. Inserir histórias com progresso
    print(f"📥 Inserindo {total} histórias no Qdrant...\n")
    
    success_count = 0
    failed_count = 0
    
    for idx, story in enumerate(stories, start=1):
        # Barra de progresso
        print_progress_bar(
            idx, 
            total, 
            prefix=f'Progresso ({idx}/{total}):',
            suffix=f'✅ {success_count} | ❌ {failed_count}',
            length=40
        )
        
        # Inserir
        if insert_story(story, idx):
            success_count += 1
        else:
            failed_count += 1
            print(f"\n⚠️  Falha na história {story['story_id']}")
        
        # Rate limiting (evitar sobrecarga da API)
        time.sleep(0.5)
    
    # 4. Verificar resultado
    print("\n" + "=" * 70)
    response = requests.get(f"{QDRANT_URL}/collections/{COLLECTION_NAME}")
    
    if response.status_code == 200:
        data = response.json()
        points_count = data['result']['points_count']
        vectors_count = data['result']['vectors_count']
        
        print(f"\n📊 RESULTADO FINAL:")
        print(f"   ✅ Histórias inseridas: {success_count}")
        print(f"   ❌ Falhas: {failed_count}")
        print(f"   📦 Points no Qdrant: {points_count}")
        print(f"   🔢 Vectors no Qdrant: {vectors_count}")
        print("\n✨ Processo concluído com sucesso!")
    else:
        print(f"\n❌ Erro ao verificar collection: {response.text}")
    
    print("=" * 70)

if __name__ == "__main__":
    main()
