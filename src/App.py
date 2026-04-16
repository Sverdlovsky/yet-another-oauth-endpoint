from fastapi.responses import RedirectResponse, JSONResponse
from starlette.middleware.sessions import SessionMiddleware
from authlib.integrations.starlette_client import OAuth
from fastapi import FastAPI, Request, HTTPException
from starlette.responses import Response
from starlette.config import Config
import jwt, json, datetime, os
import psycopg


JWT_EXP = datetime.timedelta(hours=12)

config = Config()

app = FastAPI()
app.add_middleware(SessionMiddleware, secret_key=config('SECRET_KEY'))
config('JWT_SECRET')

oauth = OAuth(config)

YANDEX_CLIENT_ID = config("YANDEX_CLIENT_ID", default=None)
YANDEX_CLIENT_SECRET = config("YANDEX_CLIENT_SECRET", default=None)
if YANDEX_CLIENT_ID and YANDEX_CLIENT_SECRET:
    oauth.register(
        name='yandex',
        client_id=YANDEX_CLIENT_ID,
        client_secret=YANDEX_CLIENT_SECRET,
        access_token_url='https://oauth.yandex.ru/token',
        authorize_url='https://oauth.yandex.ru/authorize',
        api_base_url='https://login.yandex.ru/info',
        client_kwargs={'scope': 'login:email login:info'},
    )

GOOGLE_CLIENT_ID = config("GOOGLE_CLIENT_ID", default=None)
GOOGLE_CLIENT_SECRET = config("GOOGLE_CLIENT_SECRET", default=None)
if GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET:
    oauth.register(
        name='google',
        client_id=GOOGLE_CLIENT_ID,
        client_secret=GOOGLE_CLIENT_SECRET,
        server_metadata_url='https://accounts.google.com/.well-known/openid-configuration',
        client_kwargs={'scope': 'openid email profile'},
    )

GITHUB_CLIENT_ID = config("GITHUB_CLIENT_ID", default=None)
GITHUB_CLIENT_SECRET = config("GITHUB_CLIENT_SECRET", default=None)
if GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET:
    oauth.register(
        name='github',
        client_id=GITHUB_CLIENT_ID,
        client_secret=GITHUB_CLIENT_SECRET,
        access_token_url='https://github.com/login/oauth/access_token',
        access_token_params=None,
        authorize_url='https://github.com/login/oauth/authorize',
        authorize_params=None,
        api_base_url='https://api.github.com/',
        client_kwargs={'scope': 'read:user user:email'},
    )


@app.get('/with/{provider}')
async def login(request: Request, provider: str, next: str = f'https://{config('DOMAIN')}/'):
    client = oauth.create_client(provider)
    if not client:
        raise HTTPException(status_code=404, detail='Unknown provider')

    request.session['next'] = next
    redirect_uri = request.url_for('auth', provider=provider)
    return await client.authorize_redirect(request, redirect_uri)


@app.get('/with/{provider}/callback')
async def auth(request: Request, provider: str):
    client = oauth.create_client(provider)
    if not client:
        raise HTTPException(status_code=404, detail='Unknown provider')

    token = await client.authorize_access_token(request)

    match provider:
        case 'yandex':
            resp = await client.get('', token=token)
            user_info = resp.json()
        case 'google':
            user_info = token.get('userinfo')
        case 'github':
            resp = await client.get('user', token=token)
            user_info = resp.json()
        case _:
            raise HTTPException(status_code=400, detail='Unsupported provider')

    payload = {
        'sub': user_info.get('email') or user_info.get('login'),
        'name': user_info.get('name', ''),
        'exp': datetime.datetime.utcnow() + JWT_EXP
    }

    DSN = config("DATABASE_URL", default=None)
    if DSN:
        with psycopg.connect(DSN) as conn:
            with conn.cursor() as cur:
                cur.execute((
                    'INSERT INTO users (email, name) '
                    'VALUES (%s, %s) '
                    'ON CONFLICT DO NOTHING'
                ), ( payload['sub'], payload['name'] ))

    jwt_token = jwt.encode(payload, config('JWT_SECRET'), algorithm='HS256')

    next_url = request.session.pop('next', f'https://{config('DOMAIN')}/')

    response = RedirectResponse(url=next_url)
    response.set_cookie(
        key='access_token',
        value=jwt_token,
        httponly=True,
        domain='.' + config('DOMAIN'),
        secure=True,
        samesite="none"
    )
    return response

