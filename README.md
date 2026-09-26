# Promo Service

Microservice **Go** propriétaire des **codes promo** et de leurs **utilisations**.
Le siège seul crée les codes (réseau entier, pas de franchisé) ; `order-service`
lui délègue la validation et la consommation d'un code au moment du paiement.

| | |
|---|---|
| **Langage / techno** | Go 1.26, chi (routeur), pgx (PostgreSQL), zerolog, golang-migrate |
| **Base de données** | PostgreSQL (port hôte `5440`) |
| **Port HTTP** | `8092` |

---

## Architecture — Clean / Hexagonale

```
cmd/main.go                # Démarrage, migrations, injection des dépendances
internal/
├── domain/                # PromoCode, Redemption, erreurs typées,
│                          # port PromoRepository
├── application/           # Cas d'usage : créer/lister/modifier/supprimer (siège),
│                          # prévisualiser et consommer un code (tout authentifié)
├── adapter/
│   ├── http/              # Routeur chi, middleware JWT, DTO
│   └── postgres/          # Repository pgx + migrations SQL embarquées
└── config/                # Configuration typée depuis l'environnement
```

---

## Fonctionnalités

- **Gestion des codes** (siège uniquement, `admin`) : création, liste, modification,
  suppression — un code déjà utilisé ne peut pas être supprimé (désactiver plutôt)
- **Remise en pourcentage** avec, au choix : montant minimum de commande, plafond
  d'utilisations, fenêtre de validité (`starts_at`/`expires_at`)
- **Prévisualisation** (`GET /preview`) : vérifie l'éligibilité et calcule la remise
  **sans rien consommer** — appelable autant de fois que nécessaire pendant que le
  client compose son panier
- **Consommation atomique** (`POST /redeem`) : verrou de ligne (`SELECT ... FOR
  UPDATE`) + contrainte unique `(promo_code_id, order_id)` + `INSERT ... ON
  CONFLICT DO NOTHING` — un retry sur la même commande ne brûle jamais deux fois
  la même utilisation
- **Code immuable** : modifier un code ne change jamais son libellé — désactiver et
  recréer si besoin, pour que l'historique des commandes reste lisible

### Séparation preview / redeem

Comme `payment-service` sépare l'intention du paiement de sa confirmation,
`promo-service` sépare la **prévisualisation** (répétable, sans effet) de la
**consommation réelle** (atomique, une seule fois). `order-service` prévisualise
à la création de la commande et ne consomme qu'après succès du paiement — un
client qui abandonne son panier ne brûle jamais une utilisation.

---

## Endpoints

| Méthode | Route | Accès |
|---|---|---|
| POST | `/api/promos` | `admin` (siège) |
| GET | `/api/promos` | `admin` (siège) |
| PUT | `/api/promos/{id}` | `admin` (siège) |
| DELETE | `/api/promos/{id}` | `admin` (siège) |
| GET | `/api/promos/preview?code=&amountCents=` | authentifié |
| POST | `/api/promos/redeem` | authentifié (appelé par `order-service`) |
| GET | `/healthz`, `/readyz` | public (sondes) |

**Appel depuis `order-service`** : le JWT du client est transmis tel quel, pour
que la consommation reste attribuable à la personne qui commande (pas de compte
de service).

---

## Dépendances

> **Légende** — 🔴 indispensable (le service ne démarre pas ou ne sert à rien) ·
> 🟠 nécessaire à une fonctionnalité (le reste continue de marcher) ·
> 🟡 optionnelle (dégradation silencieuse, journalisée)

| Dépendance | Type | Conséquence si absente |
|---|---|---|
| **PostgreSQL** (`promo-db`) | 🔴 | Le service ne démarre pas |
| **auth-service** | 🟠 | Aucun appel réseau, mais toutes les routes exigent un jeton valide |

**Aucun appel sortant vers un autre service Good Food.** Ce service est appelé,
il n'appelle personne.

### Qui dépend de ce service

| Service | Type | Conséquence si `promo-service` est arrêté |
|---|---|---|
| `order-service` | 🟡 | Un `promo_code` envoyé à la création de commande est refusé (`400`) ; une commande sans code fonctionne normalement |
| `web-app` | 🟡 | Le champ code promo du panier échoue ; le reste du checkout fonctionne |

---

## Lancement

```bash
docker network create microservices-net   # une seule fois, partagé
cp .env.example .env                      # renseigner POSTGRES_PASSWORD et JWT_SECRET
docker compose up -d --build
```

⚠️ `JWT_SECRET` doit être **identique** à celui de `auth-service`.

### Variables d'environnement

| Variable | Requis | Description |
|---|---|---|
| `PORT` | non (8092) | Port HTTP |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | oui | Base dédiée `promo-db` |
| `DATABASE_URL` | oui | Chaîne pgx (le compose la construit pour le conteneur) |
| `JWT_SECRET` | oui | Secret HS256 partagé avec `auth-service` |

---

## Tests

```bash
go test ./internal/... -cover
go vet ./... && gofmt -l .
```

Couvre l'idempotence de la consommation, le contrôle d'accès siège-uniquement, le
montant minimum de commande, le plafond d'utilisations et le refus de suppression
d'un code déjà utilisé. Aucune base ni appel réseau requis.

> ⚠️ **Aucune CI n'est configurée sur ce projet** — les tests doivent être lancés
> manuellement.
