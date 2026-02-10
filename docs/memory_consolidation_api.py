# memory_consolidation_api.py
# Endpoint FastAPI para Consolidação de Memória da EVA-Mind
# Integra com o motor Go via gRPC para gerenciamento de Subespaço de Krylov
#
# Por: Junior (Criador do Projeto EVA)
# Arquitetura: eva-backend (FastAPI/Python)

from fastapi import APIRouter, HTTPException, BackgroundTasks
from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any
import asyncio
import grpc
from datetime import datetime, timedelta
import logging

# Imports do protocolo gRPC (gerado do .proto)
from protos import krylov_pb2, krylov_pb2_grpc

# Configuração de logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# ═══════════════════════════════════════════════════════════════════
# MODELOS DE DADOS (Pydantic)
# ═══════════════════════════════════════════════════════════════════

class MemoryInput(BaseModel):
    """Entrada de nova memória para a EVA"""
    embedding: List[float] = Field(..., min_items=1536, max_items=1536)
    content: str = Field(..., min_length=1)
    metadata: Optional[Dict[str, Any]] = None
    timestamp: Optional[datetime] = None
    
    class Config:
        schema_extra = {
            "example": {
                "embedding": [0.123] * 1536,
                "content": "A EVA-Mind usa aprendizado contínuo",
                "metadata": {"topic": "AI", "importance": "high"},
                "timestamp": "2026-02-10T14:30:00"
            }
        }

class MemoryResponse(BaseModel):
    """Resposta de operação de memória"""
    memory_id: str
    compressed_dimension: int
    compression_ratio: float
    orthogonality_error: float
    status: str
    processing_time_us: int

class ConsolidationConfig(BaseModel):
    """Configuração para consolidação de memória"""
    force_reorthogonalization: bool = False
    checkpoint_path: Optional[str] = None
    cleanup_old_memories: bool = False
    max_age_hours: Optional[int] = None

class KrylovStatistics(BaseModel):
    """Estatísticas do sistema Krylov"""
    dimension: int
    subspace_size: int
    window_size: int
    queue_fill: int
    total_updates: int
    last_update: str
    avg_update_time_us: int
    orthogonality_error: float
    compression_ratio: float
    memory_reduction_percent: float
    reconstruction_error: Optional[float] = None
    status: str

# ═══════════════════════════════════════════════════════════════════
# CLIENTE GRPC PARA COMUNICAÇÃO COM GO
# ═══════════════════════════════════════════════════════════════════

class KrylovGRPCClient:
    """Cliente para comunicação com o serviço Krylov em Go"""
    
    def __init__(self, server_address: str = "localhost:50051"):
        self.server_address = server_address
        self.channel = None
        self.stub = None
        self._connect()
    
    def _connect(self):
        """Estabelece conexão com o servidor Go"""
        try:
            self.channel = grpc.insecure_channel(self.server_address)
            self.stub = krylov_pb2_grpc.KrylovServiceStub(self.channel)
            logger.info(f"Conectado ao serviço Krylov em {self.server_address}")
        except Exception as e:
            logger.error(f"Erro ao conectar ao serviço Krylov: {e}")
            raise
    
    async def update_subspace(self, embedding: List[float]) -> Dict[str, Any]:
        """Adiciona nova memória ao subespaço de Krylov"""
        try:
            request = krylov_pb2.UpdateSubspaceRequest(embedding=embedding)
            response = self.stub.UpdateSubspace(request)
            
            return {
                "success": response.success,
                "compressed_embedding": list(response.compressed_embedding),
                "orthogonality_error": response.orthogonality_error,
                "processing_time_us": response.processing_time_us,
                "is_redundant": response.is_redundant
            }
        except grpc.RpcError as e:
            logger.error(f"Erro RPC ao atualizar subespaço: {e}")
            raise HTTPException(status_code=503, detail="Serviço Krylov indisponível")
    
    async def compress_vector(self, embedding: List[float]) -> List[float]:
        """Comprime um vetor usando o subespaço atual"""
        try:
            request = krylov_pb2.CompressRequest(embedding=embedding)
            response = self.stub.Compress(request)
            return list(response.compressed_embedding)
        except grpc.RpcError as e:
            logger.error(f"Erro RPC ao comprimir vetor: {e}")
            raise HTTPException(status_code=503, detail="Serviço Krylov indisponível")
    
    async def get_statistics(self) -> KrylovStatistics:
        """Obtém estatísticas do sistema Krylov"""
        try:
            request = krylov_pb2.Empty()
            response = self.stub.GetStatistics(request)
            
            return KrylovStatistics(
                dimension=response.dimension,
                subspace_size=response.subspace_size,
                window_size=response.window_size,
                queue_fill=response.queue_fill,
                total_updates=response.total_updates,
                last_update=response.last_update,
                avg_update_time_us=response.avg_update_time_us,
                orthogonality_error=response.orthogonality_error,
                compression_ratio=response.compression_ratio,
                memory_reduction_percent=response.memory_reduction_percent,
                reconstruction_error=response.reconstruction_error if response.reconstruction_error > 0 else None,
                status=response.status
            )
        except grpc.RpcError as e:
            logger.error(f"Erro RPC ao obter estatísticas: {e}")
            raise HTTPException(status_code=503, detail="Serviço Krylov indisponível")
    
    async def consolidate_memory(self, config: ConsolidationConfig) -> Dict[str, Any]:
        """Executa consolidação de memória"""
        try:
            request = krylov_pb2.ConsolidationRequest(
                force_reorthogonalization=config.force_reorthogonalization,
                checkpoint_path=config.checkpoint_path or "",
                cleanup_old_memories=config.cleanup_old_memories,
                max_age_hours=config.max_age_hours or 0
            )
            response = self.stub.ConsolidateMemory(request)
            
            return {
                "success": response.success,
                "operations_performed": list(response.operations_performed),
                "new_orthogonality_error": response.new_orthogonality_error,
                "memories_removed": response.memories_removed,
                "consolidation_time_ms": response.consolidation_time_ms
            }
        except grpc.RpcError as e:
            logger.error(f"Erro RPC ao consolidar memória: {e}")
            raise HTTPException(status_code=503, detail="Serviço Krylov indisponível")
    
    def close(self):
        """Fecha a conexão gRPC"""
        if self.channel:
            self.channel.close()

# ═══════════════════════════════════════════════════════════════════
# ROTEADOR DA API
# ═══════════════════════════════════════════════════════════════════

router = APIRouter(prefix="/api/v1/memory", tags=["Memory Management"])

# Cliente gRPC global
krylov_client = KrylovGRPCClient()

# Task de consolidação automática
consolidation_task: Optional[asyncio.Task] = None

# ═══════════════════════════════════════════════════════════════════
# ENDPOINTS
# ═══════════════════════════════════════════════════════════════════

@router.post("/add", response_model=MemoryResponse)
async def add_memory(memory: MemoryInput):
    """
    Adiciona uma nova memória ao sistema EVA-Mind.
    
    Este endpoint:
    1. Envia o embedding para o motor Go
    2. O Go executa Rank-1 Update com Gram-Schmidt
    3. Retorna o embedding comprimido (1536D → 64D)
    4. Armazena no Qdrant usando a versão comprimida
    
    Complexidade: O(n·k) = O(1536·64) ≈ 100K operações (microssegundos)
    """
    start_time = datetime.now()
    
    try:
        # Atualiza o subespaço de Krylov
        result = await krylov_client.update_subspace(memory.embedding)
        
        if result["is_redundant"]:
            logger.info("Memória redundante detectada - já presente no subespaço")
        
        # Gera ID único para a memória
        memory_id = f"mem_{int(start_time.timestamp() * 1000)}"
        
        # TODO: Armazenar no Qdrant usando embedding comprimido
        # await qdrant_client.upsert(...)
        
        processing_time = (datetime.now() - start_time).total_seconds() * 1_000_000
        
        return MemoryResponse(
            memory_id=memory_id,
            compressed_dimension=len(result["compressed_embedding"]),
            compression_ratio=len(memory.embedding) / len(result["compressed_embedding"]),
            orthogonality_error=result["orthogonality_error"],
            status="stored" if not result["is_redundant"] else "redundant",
            processing_time_us=int(processing_time)
        )
        
    except Exception as e:
        logger.error(f"Erro ao adicionar memória: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.get("/statistics", response_model=KrylovStatistics)
async def get_memory_statistics():
    """
    Obtém estatísticas detalhadas do sistema de memória Krylov.
    
    Retorna:
    - Dimensões do subespaço
    - Taxa de compressão
    - Erro de ortogonalidade
    - Performance de atualizações
    - Status de saúde do sistema
    """
    try:
        stats = await krylov_client.get_statistics()
        return stats
    except Exception as e:
        logger.error(f"Erro ao obter estatísticas: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/consolidate")
async def consolidate_memory(
    config: ConsolidationConfig,
    background_tasks: BackgroundTasks
):
    """
    Executa consolidação de memória da EVA.
    
    A consolidação:
    1. Verifica erro de ortogonalidade
    2. Reortogonaliza se necessário (QR decomposition)
    3. Remove memórias antigas (opcional)
    4. Salva checkpoint do subespaço (opcional)
    
    Este processo garante que o subespaço de Krylov permaneça
    matematicamente sólido ao longo do tempo.
    """
    try:
        result = await krylov_client.consolidate_memory(config)
        
        return {
            "status": "success",
            "message": "Consolidação de memória concluída",
            "details": result,
            "timestamp": datetime.now().isoformat()
        }
        
    except Exception as e:
        logger.error(f"Erro ao consolidar memória: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/compress")
async def compress_embedding(embedding: List[float]):
    """
    Comprime um embedding sem adicioná-lo à memória.
    
    Útil para queries de busca onde você quer comprimir
    a consulta do usuário antes de buscar no Qdrant.
    
    Exemplo:
    ```
    query = "O que a EVA sabe sobre IA?"
    embedding = openai.embeddings.create(query)
    compressed = await compress_embedding(embedding)
    results = qdrant.search(compressed, top_k=10)
    ```
    """
    try:
        if len(embedding) != 1536:
            raise HTTPException(
                status_code=400, 
                detail=f"Embedding deve ter 1536 dimensões, recebido {len(embedding)}"
            )
        
        compressed = await krylov_client.compress_vector(embedding)
        
        return {
            "original_dimension": len(embedding),
            "compressed_dimension": len(compressed),
            "compressed_embedding": compressed,
            "compression_ratio": len(embedding) / len(compressed)
        }
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Erro ao comprimir embedding: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.get("/health")
async def health_check():
    """
    Verifica a saúde do sistema de memória.
    
    Status possíveis:
    - healthy: Sistema funcionando perfeitamente
    - degraded: Erro de ortogonalidade alto (>5%)
    - warning: Buffer muito cheio ou performance degradada
    - critical: Serviço Go indisponível
    """
    try:
        stats = await krylov_client.get_statistics()
        
        health_status = "healthy"
        warnings = []
        
        # Verifica erro de ortogonalidade
        if stats.orthogonality_error > 0.1:
            health_status = "critical"
            warnings.append("Erro de ortogonalidade crítico - reortogonalização urgente")
        elif stats.orthogonality_error > 0.05:
            health_status = "degraded"
            warnings.append("Erro de ortogonalidade elevado - considerar reortogonalização")
        
        # Verifica performance
        if stats.avg_update_time_us > 10000:  # >10ms
            if health_status == "healthy":
                health_status = "warning"
            warnings.append("Tempo de atualização elevado")
        
        # Verifica status do subespaço
        if stats.status != "healthy":
            if health_status == "healthy":
                health_status = "warning"
            warnings.append(f"Status do subespaço: {stats.status}")
        
        return {
            "status": health_status,
            "timestamp": datetime.now().isoformat(),
            "statistics": stats,
            "warnings": warnings if warnings else None
        }
        
    except Exception as e:
        logger.error(f"Health check falhou: {e}")
        return {
            "status": "critical",
            "timestamp": datetime.now().isoformat(),
            "error": str(e)
        }

# ═══════════════════════════════════════════════════════════════════
# CONSOLIDAÇÃO AUTOMÁTICA EM BACKGROUND
# ═══════════════════════════════════════════════════════════════════

async def auto_consolidation_task():
    """
    Task de background que executa consolidação automática periodicamente.
    
    Configuração padrão:
    - Intervalo: a cada 1 hora
    - Reortogonalização: se erro > 5%
    - Checkpoint: a cada 6 horas
    """
    logger.info("Iniciando task de consolidação automática")
    
    checkpoint_interval = timedelta(hours=6)
    last_checkpoint = datetime.now()
    
    while True:
        try:
            await asyncio.sleep(3600)  # 1 hora
            
            logger.info("Executando consolidação automática...")
            
            # Verifica se precisa de checkpoint
            needs_checkpoint = (datetime.now() - last_checkpoint) >= checkpoint_interval
            
            config = ConsolidationConfig(
                force_reorthogonalization=False,  # Apenas se necessário
                checkpoint_path="/opt/eva-mind/checkpoints/krylov.ckpt" if needs_checkpoint else None,
                cleanup_old_memories=False
            )
            
            result = await krylov_client.consolidate_memory(config)
            
            if needs_checkpoint and result["success"]:
                last_checkpoint = datetime.now()
                logger.info("Checkpoint de memória salvo")
            
            logger.info(f"Consolidação automática concluída: {result}")
            
        except Exception as e:
            logger.error(f"Erro na consolidação automática: {e}")

@router.on_event("startup")
async def startup_event():
    """Inicia a task de consolidação automática ao subir o servidor"""
    global consolidation_task
    consolidation_task = asyncio.create_task(auto_consolidation_task())
    logger.info("Sistema de memória EVA-Mind iniciado")

@router.on_event("shutdown")
async def shutdown_event():
    """Cancela a task de consolidação ao desligar o servidor"""
    global consolidation_task
    if consolidation_task:
        consolidation_task.cancel()
        try:
            await consolidation_task
        except asyncio.CancelledError:
            pass
    
    krylov_client.close()
    logger.info("Sistema de memória EVA-Mind finalizado")

# ═══════════════════════════════════════════════════════════════════
# EXEMPLO DE INTEGRAÇÃO COM A APLICAÇÃO PRINCIPAL
# ═══════════════════════════════════════════════════════════════════

"""
# main.py (FastAPI app principal)

from fastapi import FastAPI
from memory_consolidation_api import router as memory_router

app = FastAPI(title="EVA-Mind Backend")

# Inclui o roteador de memória
app.include_router(memory_router)

# Outros roteadores...
# app.include_router(chat_router)
# app.include_router(knowledge_router)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
"""
