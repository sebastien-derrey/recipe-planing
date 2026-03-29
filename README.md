# Recipe Manager

Application de gestion de recettes avec planning hebdomadaire, liste de courses automatique et intégration MCP pour Claude Desktop.

## Fonctionnalités

- Gestion de recettes avec ingrédients, quantités et photos
- Planning hebdomadaire (petit-déjeuner, déjeuner, dîner)
- Liste de courses générée automatiquement selon le nombre de personnes
- API REST complète
- Serveur MCP pour ajouter des recettes via Claude Desktop (depuis un texte, une image ou une vidéo YouTube)

---

## Installation

### 1. Prérequis

- [Go 1.22+](https://go.dev/dl/)
- [Claude Desktop](https://claude.ai/download) (pour le MCP)

### 2. Cloner / copier le projet

```bash
cd C:\Users\sebas\apps\recipe_manager
```

### 3. Configurer l'application

Copier le fichier exemple et le remplir :

```bash
cp config.json.example config.json
```

Contenu de `config.json` :

```json
{
  "port": 8080,
  "db_path": "recipe_manager.db",
  "username": "votre_identifiant",
  "password": "votre_mot_de_passe",
  "jwt_secret": "un-secret-aleatoire-de-32-caracteres-minimum",
  "anthropic_api_key": "sk-ant-xxxx",
  "mcp_service_token": "un-token-secret-long-pour-le-mcp"
}
```

| Champ | Description |
|-------|-------------|
| `port` | Port du serveur web (défaut : 8080) |
| `db_path` | Chemin vers la base de données SQLite |
| `username` | Identifiant de connexion |
| `password` | Mot de passe de connexion |
| `jwt_secret` | Clé secrète pour les tokens de session (min. 32 caractères) — [voir génération ci-dessous](#générer-jwt_secret-et-mcp_service_token) |
| `anthropic_api_key` | Clé API Anthropic (pour le MCP) — [obtenir ici](https://console.anthropic.com/) |
| `mcp_service_token` | Token secret partagé entre le MCP et l'API — [voir génération ci-dessous](#générer-jwt_secret-et-mcp_service_token) |

### Générer `jwt_secret` et `mcp_service_token`

Ces deux valeurs sont des chaînes de caractères aléatoires secrètes que vous devez générer vous-même. Lancez ces commandes dans PowerShell :

```powershell
# jwt_secret (32 octets en base64)
[Convert]::ToBase64String((1..32 | ForEach-Object { [byte](Get-Random -Max 256) }))

# mcp_service_token (64 octets en base64)
[Convert]::ToBase64String((1..64 | ForEach-Object { [byte](Get-Random -Max 256) }))
```

Vous obtiendrez quelque chose comme :

```
jwt_secret        → "K7mP2xQvL9nR4wYjF8dZ0sBhT1cEuA3N"
mcp_service_token → "X9kL2mQ7vR4wY1nF8dZ0sB3hT6cEuA5NpJ2xQ..."
```

Collez ces valeurs dans votre `config.json`.

> **Important :** ne partagez jamais ces valeurs — elles servent à signer les sessions et sécuriser les appels MCP.

### 4. Compiler le binaire

```bash
go build -o recipe_manager.exe .
```

---

## Lancement

### Serveur web

```bash
cd C:\Users\sebas\apps\recipe_manager
recipe_manager.exe
```

L'application est accessible sur [http://localhost:8080](http://localhost:8080).

---

## Utilisation du MCP avec Claude Desktop

Le MCP permet à Claude Desktop d'ajouter des recettes directement dans l'application depuis un texte, une image ou une vidéo YouTube.

### 1. Prérequis MCP

- `anthropic_api_key` et `mcp_service_token` renseignés dans `config.json`
- Le serveur web `recipe_manager.exe` doit tourner en arrière-plan
- Claude Desktop installé

### 2. Configuration Claude Desktop

Le fichier `claude_desktop_config.json` (situé dans `C:\Users\sebas\AppData\Roaming\Claude\`) est déjà configuré :

```json
{
  "mcpServers": {
    "recipe_manager": {
      "command": "C:\\Users\\sebas\\apps\\recipe_manager\\recipe_manager.exe",
      "args": ["--mcp"]
    }
  }
}
```

### 3. Lancer les deux processus

```
# Terminal 1 — serveur web (doit rester ouvert)
cd C:\Users\sebas\apps\recipe_manager
recipe_manager.exe

# Claude Desktop lance automatiquement recipe_manager.exe --mcp en arrière-plan
```

### 4. Redémarrer Claude Desktop

Claude Desktop détectera le serveur MCP au démarrage. Les outils seront disponibles dans toutes vos conversations.

### 5. Outils disponibles

| Outil | Description | Exemple |
|-------|-------------|---------|
| `add_recipe_from_text` | Extraire et sauvegarder une recette depuis du texte | *"Ajoute cette recette : [texte]"* |
| `add_recipe_from_image_url` | Extraire une recette depuis une image en ligne | *"Ajoute la recette depuis cette image : https://..."* |
| `add_recipe_from_video` | Extraire une recette depuis une vidéo YouTube | *"Ajoute la recette de cette vidéo : https://youtube.com/..."* |
| `list_recipes` | Lister toutes les recettes sauvegardées | *"Montre-moi mes recettes"* |
| `get_weekly_plan` | Voir le planning et la liste de courses d'une semaine | *"Quel est mon planning pour la semaine du 30 mars ?"* |

### 6. Exemples de demandes dans Claude Desktop

```
"Ajoute cette recette pour l'utilisateur local-user : [colle le texte d'une recette]"

"Ajoute la recette depuis cette image pour local-user : https://exemple.com/photo.jpg"

"Ajoute la recette de cette vidéo YouTube pour local-user : https://youtube.com/watch?v=..."

"Liste les recettes de local-user"

"Donne-moi le planning de la semaine du 2026-03-30 pour local-user"
```

> **Note :** `local-user` est l'identifiant interne créé automatiquement lors de la première connexion avec vos identifiants.

---

## Structure du projet

```
recipe_manager/
├── main.go                        # Point d'entrée (serveur web ou --mcp)
├── config.json                    # Configuration (à créer depuis config.json.example)
├── config.json.example            # Exemple de configuration
├── recipe_manager.db              # Base de données SQLite (créée automatiquement)
├── uploads/                       # Photos des recettes (créé automatiquement)
├── internal/
│   ├── config/                    # Chargement de la configuration
│   ├── storage/                   # Couche base de données (SQLite)
│   ├── auth/                      # Authentification JWT
│   ├── api/                       # Handlers REST
│   └── mcp/                       # Serveur MCP (stdio JSON-RPC)
└── web/                           # Frontend (HTML + Alpine.js + CSS)
```

## API REST

| Méthode | Route | Description |
|---------|-------|-------------|
| `POST` | `/auth/login` | Connexion |
| `POST` | `/auth/logout` | Déconnexion |
| `GET` | `/auth/me` | Utilisateur connecté |
| `GET` | `/api/recipes` | Liste des recettes |
| `POST` | `/api/recipes` | Créer une recette |
| `GET` | `/api/recipes/{id}` | Détail d'une recette |
| `PUT` | `/api/recipes/{id}` | Modifier une recette |
| `DELETE` | `/api/recipes/{id}` | Supprimer une recette |
| `POST` | `/api/upload` | Uploader une photo |
| `GET` | `/api/weekly-plan?week=YYYY-MM-DD` | Planning de la semaine |
| `PUT` | `/api/weekly-plan/{id}/items` | Mettre à jour le planning |
| `GET` | `/api/shopping-list?week=YYYY-MM-DD` | Liste de courses |
