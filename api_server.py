#!/usr/bin/env python3
"""
EVA-Mind-FZPN - Python REST API Server
SPRINT 7 - Integration Layer
Integra com Go Integration Microservice
"""

from fastapi import FastAPI, Depends, HTTPException, status, Header
from fastapi.security import OAuth2PasswordBearer, OAuth2PasswordRequestForm
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime, timedelta
import psycopg2
from psycopg2.extras import RealDictCursor
import bcrypt
import jwt
import httpx
import os
from dotenv import load_dotenv

# Load environment
load_dotenv()

# Config
SECRET_KEY = os.getenv("JWT_SECRET_KEY", "your-secret-key-change-in-production")
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_HOURS = 1

DB_CONFIG = {
    "host": os.getenv("DB_HOST", "104.248.219.200"),
    "port": int(os.getenv("DB_PORT", "5432")),
    "database": os.getenv("DB_NAME", "eva-db"),
    "user": os.getenv("DB_USER", "postgres"),
    "password": os.getenv("DB_PASSWORD", "Debian23@")
}

GO_SERVICE_URL = os.getenv("GO_SERVICE_URL", "http://localhost:8081")

# FastAPI app
app = FastAPI(
    title="EVA-Mind Integration API",
    description="REST API for EVA-Mind-FZPN (SPRINT 7)",
    version="1.0.0"
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# OAuth2
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="oauth/token")

# Pydantic Models
class TokenResponse(BaseModel):
    access_token: str
    token_type: str
    expires_in: int

class APIClient(BaseModel):
    id: str
    client_name: str
    client_type: str
    scopes: List[str]
    is_active: bool

class Patient(BaseModel):
    id: int
    name: str
    date_of_birth: str
    age: int
    gender: str
    email: Optional[str] = None
    phone: Optional[str] = None

class Assessment(BaseModel):
    id: str
    patient_id: int
    assessment_type: str
    total_score: Optional[int] = None
    severity: Optional[str] = None
    completed_at: datetime

# Database helpers
def get_db_connection():
    """Get database connection"""
    return psycopg2.connect(**DB_CONFIG, cursor_factory=RealDictCursor)

def create_access_token(data: dict):
    """Create JWT access token"""
    to_encode = data.copy()
    expire = datetime.utcnow() + timedelta(hours=ACCESS_TOKEN_EXPIRE_HOURS)
    to_encode.update({"exp": expire})
    encoded_jwt = jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)
    return encoded_jwt

async def get_current_client(token: str = Depends(oauth2_scheme)) -> dict:
    """Validate JWT token and return client info"""
    credentials_exception = HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="Could not validate credentials",
        headers={"WWW-Authenticate": "Bearer"},
    )

    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
        client_id: str = payload.get("client_id")
        if client_id is None:
            raise credentials_exception

        # Check if token is revoked
        conn = get_db_connection()
        cur = conn.cursor()

        cur.execute("""
            SELECT at.id, at.client_id, at.scopes, at.is_revoked,
                   ac.client_name, ac.is_active, ac.is_approved
            FROM api_tokens at
            JOIN api_clients ac ON ac.id = at.client_id
            WHERE at.access_token = %s
        """, (token,))

        token_data = cur.fetchone()
        cur.close()
        conn.close()

        if not token_data or token_data['is_revoked']:
            raise credentials_exception

        if not token_data['is_active'] or not token_data['is_approved']:
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN,
                detail="Client not active or not approved"
            )

        return dict(token_data)

    except jwt.ExpiredSignatureError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token expired"
        )
    except jwt.JWTError:
        raise credentials_exception

async def check_rate_limit(client_id: str):
    """Check if client exceeded rate limit"""
    conn = get_db_connection()
    cur = conn.cursor()

    # Count requests in last minute
    cur.execute("""
        SELECT COUNT(*) as count
        FROM api_request_logs
        WHERE client_id = %s
        AND timestamp > NOW() - INTERVAL '1 minute'
    """, (client_id,))

    count_last_minute = cur.fetchone()['count']

    # Get rate limit
    cur.execute("""
        SELECT rate_limit_per_minute
        FROM api_clients
        WHERE id = %s
    """, (client_id,))

    client = cur.fetchone()
    cur.close()
    conn.close()

    if count_last_minute >= client['rate_limit_per_minute']:
        raise HTTPException(
            status_code=status.HTTP_429_TOO_MANY_REQUESTS,
            detail="Rate limit exceeded"
        )

async def log_api_request(client_id: str, method: str, endpoint: str, status_code: int, response_time_ms: int):
    """Log API request to audit table"""
    conn = get_db_connection()
    cur = conn.cursor()

    cur.execute("""
        INSERT INTO api_request_logs (client_id, http_method, endpoint, http_status_code, response_time_ms, timestamp)
        VALUES (%s, %s, %s, %s, %s, NOW())
    """, (client_id, method, endpoint, status_code, response_time_ms))

    conn.commit()
    cur.close()
    conn.close()

# ============================================================================
# AUTHENTICATION ENDPOINTS
# ============================================================================

@app.post("/oauth/token", response_model=TokenResponse, tags=["Authentication"])
async def login(form_data: OAuth2PasswordRequestForm = Depends()):
    """
    OAuth2 Client Credentials Flow

    - **client_id**: API client ID
    - **client_secret**: API client secret
    """
    conn = get_db_connection()
    cur = conn.cursor()

    # Get client
    cur.execute("""
        SELECT id, client_name, client_secret_hash, scopes, is_active, is_approved
        FROM api_clients
        WHERE client_id = %s
    """, (form_data.username,))  # OAuth2PasswordRequestForm usa 'username' para client_id

    client = cur.fetchone()

    if not client:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid client credentials"
        )

    # Verify password
    if not bcrypt.checkpw(form_data.password.encode(), client['client_secret_hash'].encode()):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid client credentials"
        )

    if not client['is_active'] or not client['is_approved']:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Client not active or not approved"
        )

    # Create JWT token
    access_token = create_access_token(
        data={
            "client_id": str(client['id']),
            "scopes": client['scopes']
        }
    )

    # Save token to database
    expires_at = datetime.utcnow() + timedelta(hours=ACCESS_TOKEN_EXPIRE_HOURS)

    cur.execute("""
        INSERT INTO api_tokens (client_id, access_token, scopes, expires_at, created_at)
        VALUES (%s, %s, %s, %s, NOW())
    """, (client['id'], access_token, client['scopes'], expires_at))

    conn.commit()
    cur.close()
    conn.close()

    return TokenResponse(
        access_token=access_token,
        token_type="Bearer",
        expires_in=ACCESS_TOKEN_EXPIRE_HOURS * 3600
    )

# ============================================================================
# PATIENT ENDPOINTS
# ============================================================================

@app.get("/api/v1/patients/{patient_id}", response_model=Patient, tags=["Patients"])
async def get_patient(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Get patient by ID

    Requires scope: read:patients
    """
    # Check rate limit
    await check_rate_limit(current_client['client_id'])

    start_time = datetime.now()

    # Check scope
    if 'read:patients' not in current_client['scopes']:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="Insufficient permissions"
        )

    # Call Go microservice
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/serialize/patient/{patient_id}")

        if response.status_code == 404:
            raise HTTPException(status_code=404, detail="Patient not found")

        response.raise_for_status()

        # Log request
        response_time_ms = int((datetime.now() - start_time).total_seconds() * 1000)
        await log_api_request(
            current_client['client_id'],
            "GET",
            f"/api/v1/patients/{patient_id}",
            response.status_code,
            response_time_ms
        )

        return response.json()

    except httpx.HTTPError as e:
        raise HTTPException(status_code=500, detail=f"Go service error: {str(e)}")

@app.get("/api/v1/patients", tags=["Patients"])
async def list_patients(
    limit: int = 10,
    offset: int = 0,
    current_client: dict = Depends(get_current_client)
):
    """
    List patients (paginated)

    Requires scope: read:patients
    """
    await check_rate_limit(current_client['client_id'])

    if 'read:patients' not in current_client['scopes']:
        raise HTTPException(status_code=403, detail="Insufficient permissions")

    conn = get_db_connection()
    cur = conn.cursor()

    # Get patients
    cur.execute("""
        SELECT id, name,
               EXTRACT(YEAR FROM AGE(date_of_birth::date))::int as age,
               gender
        FROM patients
        ORDER BY created_at DESC
        LIMIT %s OFFSET %s
    """, (limit, offset))

    patients = cur.fetchall()

    # Get total count
    cur.execute("SELECT COUNT(*) as count FROM patients")
    total_count = cur.fetchone()['count']

    cur.close()
    conn.close()

    return {
        "data": patients,
        "page": offset // limit + 1,
        "page_size": limit,
        "total_count": total_count,
        "has_next": offset + limit < total_count
    }

# ============================================================================
# ASSESSMENT ENDPOINTS
# ============================================================================

@app.get("/api/v1/assessments/{assessment_id}", response_model=Assessment, tags=["Assessments"])
async def get_assessment(
    assessment_id: str,
    current_client: dict = Depends(get_current_client)
):
    """
    Get assessment by ID

    Requires scope: read:assessments
    """
    await check_rate_limit(current_client['client_id'])

    if 'read:assessments' not in current_client['scopes']:
        raise HTTPException(status_code=403, detail="Insufficient permissions")

    # Call Go microservice
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/serialize/assessment/{assessment_id}")

        if response.status_code == 404:
            raise HTTPException(status_code=404, detail="Assessment not found")

        response.raise_for_status()
        return response.json()

    except httpx.HTTPError as e:
        raise HTTPException(status_code=500, detail=f"Go service error: {str(e)}")

# ============================================================================
# FHIR ENDPOINTS
# ============================================================================

@app.get("/api/v1/fhir/patients/{patient_id}", tags=["FHIR"])
async def get_patient_fhir(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Get patient in FHIR R4 format

    Requires scope: read:patients or export:data
    """
    await check_rate_limit(current_client['client_id'])

    if 'read:patients' not in current_client['scopes'] and 'export:data' not in current_client['scopes']:
        raise HTTPException(status_code=403, detail="Insufficient permissions")

    # Call Go microservice
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/fhir/patient/{patient_id}")

        response.raise_for_status()
        return response.json()

    except httpx.HTTPError as e:
        raise HTTPException(status_code=500, detail=f"Go service error: {str(e)}")

@app.get("/api/v1/fhir/bundle/{patient_id}", tags=["FHIR"])
async def get_fhir_bundle(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Get FHIR Bundle for patient (Patient + Observations)

    Requires scope: export:data
    """
    await check_rate_limit(current_client['client_id'])

    if 'export:data' not in current_client['scopes']:
        raise HTTPException(status_code=403, detail="Insufficient permissions")

    # Call Go microservice
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/fhir/bundle/{patient_id}")

        response.raise_for_status()
        return response.json()

    except httpx.HTTPError as e:
        raise HTTPException(status_code=500, detail=f"Go service error: {str(e)}")

# ============================================================================
# EXPORT ENDPOINTS
# ============================================================================

@app.get("/api/v1/export/lgpd/{patient_id}", tags=["Export"])
async def export_lgpd(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Export patient data (LGPD/GDPR portability)

    Requires scope: export:data
    """
    await check_rate_limit(current_client['client_id'])

    if 'export:data' not in current_client['scopes']:
        raise HTTPException(status_code=403, detail="Insufficient permissions")

    # Call Go microservice
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/export/lgpd/{patient_id}")

        response.raise_for_status()
        return response.json()

    except httpx.HTTPError as e:
        raise HTTPException(status_code=500, detail=f"Go service error: {str(e)}")

# ============================================================================
# CLINICAL DASHBOARD ENDPOINTS (Para Clínicos/Médicos)
# ============================================================================

class PatientRisk(BaseModel):
    id: int
    name: str
    age: int
    gender: str
    risk_level: str
    phq9_score: Optional[int] = None
    gad7_score: Optional[int] = None
    cssrs_score: Optional[int] = None
    last_assessment: Optional[datetime] = None
    active_alerts: int = 0
    persona_status: Optional[str] = None

class ClinicalAlert(BaseModel):
    id: str
    patient_id: int
    patient_name: str
    alert_type: str
    severity: str
    message: str
    created_at: datetime
    acknowledged: bool = False
    acknowledged_by: Optional[str] = None
    acknowledged_at: Optional[datetime] = None

class TrajectoryPrediction(BaseModel):
    patient_id: int
    prediction_date: datetime
    risk_7d: float
    risk_30d: float
    risk_90d: float
    confidence: float
    recommended_interventions: List[str]
    trajectory_trend: str  # improving, stable, declining

class ClinicalStats(BaseModel):
    total_patients: int
    critical_risk: int
    high_risk: int
    moderate_risk: int
    low_risk: int
    pending_alerts: int
    assessments_today: int
    active_crises: int

@app.get("/api/v1/clinical/stats", response_model=ClinicalStats, tags=["Clinical Dashboard"])
async def get_clinical_stats(current_client: dict = Depends(get_current_client)):
    """
    Obter estatísticas gerais do dashboard clínico

    Requires scope: read:patients or clinical:access
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        # Total de pacientes
        cur.execute("SELECT COUNT(*) as count FROM patients")
        total_patients = cur.fetchone()['count']

        # Pacientes por nível de risco (baseado no último C-SSRS)
        cur.execute("""
            WITH latest_cssrs AS (
                SELECT DISTINCT ON (patient_id)
                    patient_id, total_score, severity_level
                FROM clinical_assessments
                WHERE scale_type = 'CSSRS'
                ORDER BY patient_id, assessed_at DESC
            )
            SELECT
                COUNT(*) FILTER (WHERE severity_level = 'critical') as critical,
                COUNT(*) FILTER (WHERE severity_level = 'high') as high,
                COUNT(*) FILTER (WHERE severity_level = 'moderate') as moderate,
                COUNT(*) FILTER (WHERE severity_level IN ('low', 'minimal', 'none')) as low
            FROM latest_cssrs
        """)
        risk_counts = cur.fetchone()

        # Alertas pendentes
        cur.execute("""
            SELECT COUNT(*) as count
            FROM alerts
            WHERE acknowledged = false AND status = 'active'
        """)
        pending_alerts = cur.fetchone()['count']

        # Avaliações hoje
        cur.execute("""
            SELECT COUNT(*) as count
            FROM clinical_assessments
            WHERE DATE(assessed_at) = CURRENT_DATE
        """)
        assessments_today = cur.fetchone()['count']

        # Crises ativas
        cur.execute("""
            SELECT COUNT(*) as count
            FROM crisis_events
            WHERE resolved_at IS NULL
        """)
        active_crises = cur.fetchone()['count']

        return ClinicalStats(
            total_patients=total_patients,
            critical_risk=risk_counts['critical'] or 0,
            high_risk=risk_counts['high'] or 0,
            moderate_risk=risk_counts['moderate'] or 0,
            low_risk=risk_counts['low'] or 0,
            pending_alerts=pending_alerts,
            assessments_today=assessments_today,
            active_crises=active_crises
        )
    finally:
        cur.close()
        conn.close()

@app.get("/api/v1/clinical/patients", tags=["Clinical Dashboard"])
async def get_clinical_patients(
    risk_filter: Optional[str] = None,
    limit: int = 50,
    offset: int = 0,
    current_client: dict = Depends(get_current_client)
):
    """
    Listar pacientes com níveis de risco para dashboard clínico

    - risk_filter: critical, high, moderate, low (opcional)
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        # Query com dados de risco
        query = """
            WITH latest_assessments AS (
                SELECT DISTINCT ON (patient_id, scale_type)
                    patient_id,
                    scale_type,
                    total_score,
                    severity_level,
                    assessed_at
                FROM clinical_assessments
                ORDER BY patient_id, scale_type, assessed_at DESC
            ),
            patient_alerts AS (
                SELECT patient_id, COUNT(*) as alert_count
                FROM alerts
                WHERE acknowledged = false AND status = 'active'
                GROUP BY patient_id
            ),
            persona_sessions AS (
                SELECT DISTINCT ON (patient_id)
                    patient_id, persona_code
                FROM persona_sessions
                ORDER BY patient_id, started_at DESC
            )
            SELECT
                p.id,
                p.name,
                EXTRACT(YEAR FROM AGE(p.date_of_birth::date))::int as age,
                p.gender,
                COALESCE(phq.total_score, 0) as phq9_score,
                COALESCE(gad.total_score, 0) as gad7_score,
                COALESCE(css.total_score, 0) as cssrs_score,
                COALESCE(css.severity_level, 'unknown') as risk_level,
                GREATEST(phq.assessed_at, gad.assessed_at, css.assessed_at) as last_assessment,
                COALESCE(pa.alert_count, 0) as active_alerts,
                ps.persona_code as persona_status
            FROM patients p
            LEFT JOIN latest_assessments phq ON p.id = phq.patient_id AND phq.scale_type = 'PHQ9'
            LEFT JOIN latest_assessments gad ON p.id = gad.patient_id AND gad.scale_type = 'GAD7'
            LEFT JOIN latest_assessments css ON p.id = css.patient_id AND css.scale_type = 'CSSRS'
            LEFT JOIN patient_alerts pa ON p.id = pa.patient_id
            LEFT JOIN persona_sessions ps ON p.id = ps.patient_id
        """

        if risk_filter:
            query += f" WHERE css.severity_level = %s"
            query += " ORDER BY CASE css.severity_level WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'moderate' THEN 3 ELSE 4 END, p.name"
            query += " LIMIT %s OFFSET %s"
            cur.execute(query, (risk_filter, limit, offset))
        else:
            query += " ORDER BY CASE css.severity_level WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'moderate' THEN 3 ELSE 4 END, p.name"
            query += " LIMIT %s OFFSET %s"
            cur.execute(query, (limit, offset))

        patients = cur.fetchall()

        return {
            "data": [dict(p) for p in patients],
            "page_size": limit,
            "offset": offset
        }
    finally:
        cur.close()
        conn.close()

@app.get("/api/v1/clinical/alerts", tags=["Clinical Dashboard"])
async def get_clinical_alerts(
    severity: Optional[str] = None,
    acknowledged: bool = False,
    limit: int = 50,
    current_client: dict = Depends(get_current_client)
):
    """
    Listar alertas clínicos

    - severity: critical, high, moderate, low
    - acknowledged: true/false
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        query = """
            SELECT
                a.id::text,
                a.patient_id,
                p.name as patient_name,
                a.alert_type,
                a.severity,
                a.message,
                a.created_at,
                a.acknowledged,
                a.acknowledged_by,
                a.acknowledged_at
            FROM alerts a
            JOIN patients p ON a.patient_id = p.id
            WHERE a.acknowledged = %s
        """
        params = [acknowledged]

        if severity:
            query += " AND a.severity = %s"
            params.append(severity)

        query += " ORDER BY CASE a.severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'moderate' THEN 3 ELSE 4 END, a.created_at DESC"
        query += " LIMIT %s"
        params.append(limit)

        cur.execute(query, params)
        alerts = cur.fetchall()

        return {"data": [dict(a) for a in alerts]}
    finally:
        cur.close()
        conn.close()

@app.post("/api/v1/clinical/alerts/{alert_id}/acknowledge", tags=["Clinical Dashboard"])
async def acknowledge_alert(
    alert_id: str,
    current_client: dict = Depends(get_current_client)
):
    """
    Reconhecer/Confirmar um alerta
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        cur.execute("""
            UPDATE alerts
            SET acknowledged = true,
                acknowledged_at = NOW(),
                acknowledged_by = %s
            WHERE id = %s
            RETURNING id
        """, (current_client['client_name'], alert_id))

        result = cur.fetchone()
        conn.commit()

        if not result:
            raise HTTPException(status_code=404, detail="Alert not found")

        return {"message": "Alert acknowledged", "alert_id": alert_id}
    finally:
        cur.close()
        conn.close()

@app.get("/api/v1/clinical/patients/{patient_id}/trajectory", tags=["Clinical Dashboard"])
async def get_patient_trajectory(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Obter predição de trajetória do paciente
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        # Buscar última simulação de trajetória
        cur.execute("""
            SELECT
                patient_id,
                simulation_date as prediction_date,
                mean_trajectory as risk_7d,
                percentile_75 as risk_30d,
                percentile_95 as risk_90d,
                confidence_interval_upper - confidence_interval_lower as confidence,
                recommended_interventions,
                CASE
                    WHEN mean_trajectory < 0.3 THEN 'improving'
                    WHEN mean_trajectory > 0.6 THEN 'declining'
                    ELSE 'stable'
                END as trajectory_trend
            FROM trajectory_simulations
            WHERE patient_id = %s
            ORDER BY simulation_date DESC
            LIMIT 1
        """, (patient_id,))

        trajectory = cur.fetchone()

        if not trajectory:
            # Retornar valores default se não houver simulação
            return {
                "patient_id": patient_id,
                "prediction_date": datetime.utcnow(),
                "risk_7d": 0.0,
                "risk_30d": 0.0,
                "risk_90d": 0.0,
                "confidence": 0.0,
                "recommended_interventions": [],
                "trajectory_trend": "unknown"
            }

        return dict(trajectory)
    finally:
        cur.close()
        conn.close()

@app.get("/api/v1/clinical/patients/{patient_id}/assessments-history", tags=["Clinical Dashboard"])
async def get_patient_assessments_history(
    patient_id: int,
    scale_type: Optional[str] = None,
    days: int = 90,
    current_client: dict = Depends(get_current_client)
):
    """
    Histórico de avaliações de um paciente
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        query = """
            SELECT
                id::text,
                scale_type,
                total_score,
                severity_level,
                assessed_at,
                assessed_by
            FROM clinical_assessments
            WHERE patient_id = %s
            AND assessed_at >= NOW() - INTERVAL '%s days'
        """
        params = [patient_id, days]

        if scale_type:
            query += " AND scale_type = %s"
            params.append(scale_type)

        query += " ORDER BY assessed_at DESC"

        cur.execute(query, params)
        assessments = cur.fetchall()

        return {"data": [dict(a) for a in assessments]}
    finally:
        cur.close()
        conn.close()

@app.get("/api/v1/clinical/patients/{patient_id}/interventions", tags=["Clinical Dashboard"])
async def get_patient_interventions(
    patient_id: int,
    current_client: dict = Depends(get_current_client)
):
    """
    Intervenções recomendadas para um paciente
    """
    await check_rate_limit(current_client['client_id'])

    conn = get_db_connection()
    cur = conn.cursor()

    try:
        cur.execute("""
            SELECT
                ri.id::text,
                ri.intervention_type,
                ri.recommendation,
                ri.urgency,
                ri.evidence_source,
                ri.status,
                ri.recommended_at,
                ri.applied_at,
                ri.outcome
            FROM recommended_interventions ri
            WHERE ri.patient_id = %s
            ORDER BY
                CASE ri.urgency WHEN 'immediate' THEN 1 WHEN 'urgent' THEN 2 WHEN 'routine' THEN 3 ELSE 4 END,
                ri.recommended_at DESC
            LIMIT 20
        """, (patient_id,))

        interventions = cur.fetchall()

        return {"data": [dict(i) for i in interventions]}
    finally:
        cur.close()
        conn.close()

# ============================================================================
# HEALTH & INFO
# ============================================================================

@app.get("/health", tags=["System"])
async def health_check():
    """Health check endpoint"""
    # Check Go service
    go_service_healthy = False
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{GO_SERVICE_URL}/health", timeout=2.0)
            go_service_healthy = response.status_code == 200
    except:
        pass

    return {
        "status": "healthy" if go_service_healthy else "degraded",
        "api_server": "healthy",
        "go_service": "healthy" if go_service_healthy else "unhealthy",
        "timestamp": datetime.utcnow()
    }

@app.get("/", tags=["System"])
async def root():
    """API info"""
    return {
        "name": "EVA-Mind Integration API",
        "version": "1.0.0",
        "sprint": "SPRINT 7 - Integration Layer",
        "docs": "/docs",
        "health": "/health"
    }

# ============================================================================
# RUN SERVER
# ============================================================================

if __name__ == "__main__":
    import uvicorn

    print("=" * 60)
    print("🚀 EVA-Mind Integration API Server")
    print("=" * 60)
    print(f"API Server: http://localhost:8000")
    print(f"API Docs: http://localhost:8000/docs")
    print(f"Go Service: {GO_SERVICE_URL}")
    print(f"Database: {DB_CONFIG['host']}:{DB_CONFIG['port']}/{DB_CONFIG['database']}")
    print("=" * 60)

    uvicorn.run(app, host="0.0.0.0", port=8000)
