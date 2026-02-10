"""
KrylovClient - Cliente Python para o gRPC KrylovService (Go)

Arquitetura:
  FastAPI (porta 8000) --gRPC--> Go KrylovService (porta 50051)
                                       |
                                       v
                                 KrylovMemoryManager

Uso:
  from krylov_client import KrylovClient

  client = KrylovClient("localhost:50051")

  # Comprimir embedding 1536D -> 64D
  compressed = client.compress(embedding_1536d)

  # Reconstruir 64D -> ~1536D
  reconstructed = client.reconstruct(compressed)

  # Batch compression
  compressed_batch = client.batch_compress([emb1, emb2, emb3])

  # Atualizar subespaco com novo embedding
  client.update_subspace(new_embedding)

  # Estatisticas
  stats = client.get_statistics()

Nota: Como o proto ainda nao foi compilado com protoc, este cliente usa
gRPC generico (UnaryUnary). Quando o proto for compilado, trocar por stubs.
"""

import grpc
import json
import struct
import time
from typing import List, Optional, Dict, Any


class KrylovClient:
    """Cliente gRPC para o KrylovService em Go."""

    def __init__(self, address: str = "localhost:50051", timeout: float = 5.0):
        self.address = address
        self.timeout = timeout
        self.channel = None
        self._connect()

    def _connect(self):
        """Estabelece conexao gRPC."""
        self.channel = grpc.insecure_channel(self.address)
        # Verificar conectividade
        try:
            grpc.channel_ready_future(self.channel).result(timeout=self.timeout)
        except grpc.FutureTimeoutError:
            raise ConnectionError(f"Timeout conectando ao KrylovService em {self.address}")

    def close(self):
        """Fecha conexao gRPC."""
        if self.channel:
            self.channel.close()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()


class KrylovClientHTTP:
    """
    Cliente HTTP alternativo para quando gRPC nao esta disponivel.
    Usa HTTP/JSON puro na porta do servidor Go.

    Este e o cliente recomendado para FastAPI ate o proto ser compilado.
    """

    def __init__(self, base_url: str = "http://localhost:50052"):
        self.base_url = base_url.rstrip("/")
        self._session = None

    def _get_session(self):
        if self._session is None:
            import httpx
            self._session = httpx.Client(timeout=10.0)
        return self._session

    def compress(self, vector: List[float]) -> List[float]:
        """Comprime embedding 1536D -> 64D."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/compress",
            json={"vector": vector},
        )
        resp.raise_for_status()
        return resp.json()["compressed"]

    def reconstruct(self, compressed: List[float]) -> List[float]:
        """Reconstroi embedding 64D -> ~1536D."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/reconstruct",
            json={"compressed": compressed},
        )
        resp.raise_for_status()
        return resp.json()["reconstructed"]

    def batch_compress(self, vectors: List[List[float]]) -> List[List[float]]:
        """Comprime multiplos embeddings."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/batch_compress",
            json={"vectors": vectors},
        )
        resp.raise_for_status()
        return resp.json()["compressed"]

    def update_subspace(self, vector: List[float]) -> Dict[str, Any]:
        """Adiciona embedding ao subespaco."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/update",
            json={"vector": vector},
        )
        resp.raise_for_status()
        return resp.json()

    def get_statistics(self) -> Dict[str, Any]:
        """Retorna estatisticas do KrylovMemoryManager."""
        resp = self._get_session().get(f"{self.base_url}/krylov/stats")
        resp.raise_for_status()
        return resp.json()

    def save_checkpoint(self, filepath: str) -> bool:
        """Salva checkpoint em disco."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/checkpoint/save",
            json={"filepath": filepath},
        )
        resp.raise_for_status()
        return resp.json().get("success", False)

    def load_checkpoint(self, filepath: str) -> bool:
        """Carrega checkpoint de disco."""
        resp = self._get_session().post(
            f"{self.base_url}/krylov/checkpoint/load",
            json={"filepath": filepath},
        )
        resp.raise_for_status()
        return resp.json().get("success", False)

    def health_check(self) -> Dict[str, Any]:
        """Verifica saude do servico."""
        resp = self._get_session().get(f"{self.base_url}/krylov/health")
        resp.raise_for_status()
        return resp.json()

    def close(self):
        if self._session:
            self._session.close()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()


# FastAPI integration example
def create_fastapi_krylov_router(krylov_url: str = "http://localhost:50052"):
    """
    Cria router FastAPI que proxeia para o KrylovService Go.

    Uso no FastAPI:
        from krylov_client import create_fastapi_krylov_router
        app = FastAPI()
        app.include_router(create_fastapi_krylov_router(), prefix="/api/v1")
    """
    try:
        from fastapi import APIRouter, HTTPException
        from pydantic import BaseModel
    except ImportError:
        raise ImportError("FastAPI e pydantic sao necessarios: pip install fastapi pydantic")

    router = APIRouter(tags=["krylov"])
    client = KrylovClientHTTP(krylov_url)

    class CompressIn(BaseModel):
        vector: List[float]

    class CompressOut(BaseModel):
        compressed: List[float]
        compression_time_us: float = 0

    class ReconstructIn(BaseModel):
        compressed: List[float]

    class ReconstructOut(BaseModel):
        reconstructed: List[float]

    class BatchIn(BaseModel):
        vectors: List[List[float]]

    class BatchOut(BaseModel):
        compressed: List[List[float]]
        total_time_us: float = 0

    class UpdateIn(BaseModel):
        vector: List[float]

    @router.post("/compress", response_model=CompressOut)
    async def compress(req: CompressIn):
        try:
            start = time.time()
            result = client.compress(req.vector)
            elapsed = (time.time() - start) * 1_000_000
            return CompressOut(compressed=result, compression_time_us=elapsed)
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.post("/reconstruct", response_model=ReconstructOut)
    async def reconstruct(req: ReconstructIn):
        try:
            result = client.reconstruct(req.compressed)
            return ReconstructOut(reconstructed=result)
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.post("/batch_compress", response_model=BatchOut)
    async def batch_compress(req: BatchIn):
        try:
            start = time.time()
            result = client.batch_compress(req.vectors)
            elapsed = (time.time() - start) * 1_000_000
            return BatchOut(compressed=result, total_time_us=elapsed)
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.post("/update")
    async def update(req: UpdateIn):
        try:
            return client.update_subspace(req.vector)
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.get("/stats")
    async def stats():
        try:
            return client.get_statistics()
        except Exception as e:
            raise HTTPException(status_code=500, detail=str(e))

    @router.get("/health")
    async def health():
        try:
            return client.health_check()
        except Exception as e:
            raise HTTPException(status_code=503, detail=str(e))

    return router
