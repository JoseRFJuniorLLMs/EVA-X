#!/usr/bin/env python3
"""
EVA Core Memory - Script de Carga de Conhecimento
Carrega conhecimento inicial de EVA a partir de arquivo JSON
"""

import json
import requests
import sys
import time
from pathlib import Path
from typing import Dict, List


class Colors:
    """Cores ANSI para output formatado"""
    GREEN = '\033[0;32m'
    BLUE = '\033[0;34m'
    RED = '\033[0;31m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'  # No Color


class EVAKnowledgeLoader:
    """Carregador de conhecimento para EVA Core Memory"""

    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url
        self.total_loaded = 0
        self.total_errors = 0

    def teach_eva(self, lesson: str, category: str, importance: float) -> bool:
        """
        Ensina uma lição para EVA

        Args:
            lesson: Conteúdo da lição
            category: Categoria (lesson, pattern, meta_insight, emotional_rule, self_critique)
            importance: Importância (0.0-1.0)

        Returns:
            True se sucesso, False se erro
        """
        try:
            payload = {
                "lesson": lesson,
                "category": category,
                "importance": importance
            }

            response = requests.post(
                f"{self.base_url}/self/teach",
                json=payload,
                headers={"Content-Type": "application/json"},
                timeout=10
            )

            if response.status_code == 201:
                truncated = lesson[:70] + "..." if len(lesson) > 70 else lesson
                print(f"  {Colors.GREEN}✅{Colors.NC} {truncated}")
                self.total_loaded += 1
                return True
            else:
                truncated = lesson[:70] + "..." if len(lesson) > 70 else lesson
                print(f"  {Colors.RED}❌{Colors.NC} Erro {response.status_code}: {truncated}")
                self.total_errors += 1
                return False

        except requests.exceptions.RequestException as e:
            print(f"  {Colors.RED}❌{Colors.NC} Erro de conexão: {str(e)[:50]}")
            self.total_errors += 1
            return False

    def load_category(self, category_name: str, items: List[Dict]) -> None:
        """
        Carrega todas as memórias de uma categoria

        Args:
            category_name: Nome da categoria para display
            items: Lista de itens da categoria
        """
        print(f"\n{Colors.BLUE}{category_name}{Colors.NC}")
        print()

        for item in items:
            self.teach_eva(
                lesson=item["content"],
                category=item["category"],
                importance=item["importance"]
            )
            time.sleep(0.1)  # Rate limiting gentil

        print()
        print(f"{Colors.GREEN}✅ {category_name} carregado{Colors.NC}")

    def verify_load(self) -> None:
        """Verifica carga e exibe estatísticas"""
        print("\n" + "=" * 50)
        print(f"\n{Colors.BLUE}🔍 Verificando carga...{Colors.NC}\n")

        # Estatísticas
        try:
            stats_response = requests.get(f"{self.base_url}/self/memories/stats", timeout=5)
            if stats_response.status_code == 200:
                stats = stats_response.json()
                print("📊 Estatísticas da Memória:")
                print(f"  Total de memórias: {stats.get('total_memories', 0)}")

                by_type = stats.get('by_type', {})
                if by_type:
                    print("  Por tipo:")
                    for mem_type, count in by_type.items():
                        print(f"    • {mem_type}: {count}")
                print()
        except requests.exceptions.RequestException as e:
            print(f"{Colors.YELLOW}⚠️  Não foi possível obter estatísticas{Colors.NC}\n")

        # Personalidade
        try:
            personality_response = requests.get(f"{self.base_url}/self/personality", timeout=5)
            if personality_response.status_code == 200:
                personality = personality_response.json()
                big_five = personality.get('big_five', {})

                print("💜 Personalidade de EVA (Big Five):")
                print(f"  • Openness (Abertura): {big_five.get('openness', 0):.2f}")
                print(f"  • Conscientiousness (Conscienciosidade): {big_five.get('conscientiousness', 0):.2f}")
                print(f"  • Extraversion (Extroversão): {big_five.get('extraversion', 0):.2f}")
                print(f"  • Agreeableness (Amabilidade): {big_five.get('agreeableness', 0):.2f}")
                print(f"  • Neuroticism (Neuroticismo): {big_five.get('neuroticism', 0):.2f}")
                print()
        except requests.exceptions.RequestException as e:
            print(f"{Colors.YELLOW}⚠️  Não foi possível obter personalidade{Colors.NC}\n")

    def print_summary(self) -> None:
        """Imprime resumo final da carga"""
        print("=" * 50)
        print(f"{Colors.GREEN}✅ CARGA INICIAL COMPLETA!{Colors.NC}")
        print()
        print("📊 Resumo:")
        print(f"  • Memórias carregadas: {self.total_loaded}")
        print(f"  • Erros encontrados: {self.total_errors}")
        print()
        print("🧠 EVA agora tem conhecimento fundamental e está pronta para:")
        print("  1. Atender pacientes com empatia e segurança")
        print("  2. Aprender continuamente através das sessões")
        print("  3. Evoluir sua personalidade com experiência")
        print()
        print("📚 Próximos passos:")
        print(f"  • Monitorar: curl {self.base_url}/self/personality")
        print(f"  • Ver memórias: curl {self.base_url}/self/memories")
        print(f"  • Buscar: curl -X POST {self.base_url}/self/memories/search -d '{{\"query\":\"ansiedade\"}}'")
        print()
        print(f"{Colors.GREEN}EVA está viva e pronta para ajudar! 🧠⚡💜{Colors.NC}")
        print("=" * 50)

    def load_from_json(self, json_path: Path) -> None:
        """
        Carrega conhecimento de arquivo JSON

        Args:
            json_path: Caminho para o arquivo JSON
        """
        # Carregar JSON
        with open(json_path, 'r', encoding='utf-8') as f:
            data = json.load(f)

        # Header
        print("\n🧠 EVA Core Memory - Carga de Conhecimento")
        print("=" * 50)
        print()
        print(f"Base URL: {self.base_url}")
        print(f"Arquivo: {json_path}")
        print()

        # Metadata
        metadata = data.get('metadata', {})
        if metadata:
            print(f"📋 Versão: {metadata.get('version', 'N/A')}")
            print(f"📅 Criado em: {metadata.get('created_at', 'N/A')}")
            print(f"📝 {metadata.get('description', 'N/A')}")
            print()

        # Carregar cada categoria
        category_map = {
            'lessons': '📚 Carregando Lições Fundamentais...',
            'patterns': '🔍 Carregando Padrões Comportamentais...',
            'emotional_rules': '💜 Carregando Regras Emocionais...',
            'meta_insights': '🌟 Carregando Meta-Insights...',
            'safety_protocols': '🚨 Carregando Protocolos de Segurança...'
        }

        for category_key, category_title in category_map.items():
            if category_key in data and data[category_key]:
                self.load_category(category_title, data[category_key])

        # Verificação e resumo
        self.verify_load()
        self.print_summary()


def main():
    """Função principal"""
    # Parse argumentos
    json_path = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("data/eva_initial_knowledge.json")
    base_url = sys.argv[2] if len(sys.argv) > 2 else "http://localhost:8080"

    # Validar arquivo
    if not json_path.exists():
        print(f"{Colors.RED}❌ Arquivo não encontrado: {json_path}{Colors.NC}")
        print()
        print("Uso:")
        print(f"  python3 {sys.argv[0]} [caminho_json] [base_url]")
        print()
        print("Exemplo:")
        print(f"  python3 {sys.argv[0]} data/eva_initial_knowledge.json http://localhost:8080")
        sys.exit(1)

    # Carregar conhecimento
    loader = EVAKnowledgeLoader(base_url)

    try:
        loader.load_from_json(json_path)
    except KeyboardInterrupt:
        print(f"\n\n{Colors.YELLOW}⚠️  Carga interrompida pelo usuário{Colors.NC}")
        sys.exit(1)
    except Exception as e:
        print(f"\n{Colors.RED}❌ Erro: {str(e)}{Colors.NC}")
        sys.exit(1)


if __name__ == "__main__":
    main()
