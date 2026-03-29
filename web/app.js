function app() {
  return {
    // State
    user: null,
    userLoaded: false,
    loginForm: { username: '', password: '' },
    loginError: '',
    loggingIn: false,
    page: 'recipes',
    recipes: [],
    loadingRecipes: false,

    // Planning
    currentWeek: mondayOf(new Date()),
    planItems: [],
    planID: null,
    days: [
      { n: 1, label: 'Lundi' },
      { n: 2, label: 'Mardi' },
      { n: 3, label: 'Mercredi' },
      { n: 4, label: 'Jeudi' },
      { n: 5, label: 'Vendredi' },
      { n: 6, label: 'Samedi' },
      { n: 7, label: 'Dimanche' },
    ],
    mealTypes: ['breakfast', 'lunch', 'dinner'],

    // Shopping
    shoppingList: [],

    // Import / share
    importing: false,
    importError: '',
    importSuccess: false,
    importUrl: '',
    pendingShare: null,
    shareLoginRequired: false,

    // Pantry
    pantry: [],
    loadingPantry: false,
    showPantryForm: false,
    editingPantryItem: null,
    savingPantry: false,
    pantryForm: { ingredient: '', quantity: 0, unit: '' },

    // Units
    units: [
      // Volume
      'ml', 'cl', 'dl', 'L',
      'cs', 'cc', 'tasse',
      // Poids
      'mg', 'g', 'kg',
      // Quantité / conditionnement
      'pièce', 'unité', 'tranche', 'portion',
      'botte', 'bouquet', 'gousse', 'cube',
      'sachet', 'boîte', 'pot', 'conserve',
      'branche', 'feuille', 'tige',
      // Petites mesures
      'pincée', 'poignée', 'filet', 'trait', 'noix',
      // Autres
      'cm',
    ],

    // Form
    showForm: false,
    editingRecipe: null,
    saving: false,
    form: emptyForm(),
    uploadingPhoto: false,
    photoError: '',
    photoDragOver: false,

    async init() {
      // Capture share params before anything else clears the URL
      const params = new URLSearchParams(window.location.search);
      const shareUrl   = params.get('share_url');
      const shareText  = params.get('share_text');
      const shareImage = params.get('share_image');
      const loginRequired = params.get('share_login_required');
      if (shareUrl || shareText || shareImage || loginRequired) {
        history.replaceState({}, '', '/');
      }
      if (loginRequired) {
        this.shareLoginRequired = true;
      } else if (shareUrl || shareText || shareImage) {
        this.pendingShare = { url: shareUrl, text: shareText, imageUrl: shareImage };
      }

      // Auth check
      try {
        const res = await fetch('/auth/me');
        if (res.ok) this.user = await res.json();
      } catch (_) {}
      this.userLoaded = true;

      if (this.user) {
        this.loadRecipes();
        if (this.pendingShare) {
          const share = this.pendingShare;
          this.pendingShare = null;
          this.importShared(share);
        }
      }
    },

    // ── Auth ────────────────────────────────────────────────────────────

    async login() {
      this.loggingIn = true;
      this.loginError = '';
      const res = await fetch('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(this.loginForm),
      });
      this.loggingIn = false;
      if (res.ok) {
        this.user = await res.json();
        this.loginForm = { username: '', password: '' };
        this.loadRecipes();
        if (this.pendingShare) {
          const share = this.pendingShare;
          this.pendingShare = null;
          this.importShared(share);
        }
      } else {
        const err = await res.json();
        this.loginError = err.error || 'Erreur de connexion';
      }
    },

    async logout() {
      await fetch('/auth/logout', { method: 'POST' });
      this.user = null;
      this.recipes = [];
    },

    // ── Recipes ─────────────────────────────────────────────────────────

    async loadRecipes() {
      this.loadingRecipes = true;
      const res = await fetch('/api/recipes');
      if (res.ok) this.recipes = await res.json();
      this.loadingRecipes = false;
    },

    openRecipeForm(recipe) {
      this.editingRecipe = recipe;
      if (recipe) {
        // Load full recipe with ingredients
        fetch(`/api/recipes/${recipe.id}`)
          .then(r => r.json())
          .then(full => {
            this.form = {
              title:        full.title,
              description:  full.description || '',
              instructions: full.instructions,
              servings:     full.servings,
              tags:         full.tags || [],
              source_url:   full.source_url || '',
              ingredients:  (full.ingredients || []).map(i => ({
                name:     i.name,
                quantity: i.quantity,
                unit:     i.unit,
                notes:    i.notes || '',
              })),
            };
          });
      } else {
        this.form = emptyForm();
      }
      this.showForm = true;
    },

    addTag(input) {
      const val = input.value.trim().toLowerCase().replace(/,/g, '');
      if (val && !this.form.tags.includes(val)) {
        this.form.tags.push(val);
      }
      input.value = '';
    },

    async uploadPhoto(file) {
      if (!file) return;
      this.uploadingPhoto = true;
      this.photoError = '';
      const fd = new FormData();
      fd.append('photo', file);
      const res = await fetch('/api/upload', { method: 'POST', body: fd });
      this.uploadingPhoto = false;
      if (res.ok) {
        const data = await res.json();
        this.form.image_url = data.url;
      } else {
        const err = await res.json();
        this.photoError = err.error || 'Erreur lors de l\'upload';
      }
    },

    onPhotoSelect(event) {
      const file = event.target.files[0];
      if (file) this.uploadPhoto(file);
    },

    onPhotoDrop(event) {
      this.photoDragOver = false;
      const file = event.dataTransfer.files[0];
      if (file) this.uploadPhoto(file);
    },

    async saveRecipe() {
      this.saving = true;
      const method = this.editingRecipe ? 'PUT' : 'POST';
      const url = this.editingRecipe ? `/api/recipes/${this.editingRecipe.id}` : '/api/recipes';
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(this.form),
      });
      this.saving = false;
      if (res.ok) {
        this.showForm = false;
        await this.loadRecipes();
      } else {
        const err = await res.json();
        alert('Erreur : ' + (err.error || 'inconnue'));
      }
    },

    async deleteRecipe(id) {
      if (!confirm('Supprimer cette recette ?')) return;
      const res = await fetch(`/api/recipes/${id}`, { method: 'DELETE' });
      if (res.ok) await this.loadRecipes();
    },

    // ── Planning ────────────────────────────────────────────────────────

    async loadPlanning() {
      const week = fmtDate(this.currentWeek);
      const res = await fetch(`/api/weekly-plan?week=${week}`);
      if (res.ok) {
        const plan = await res.json();
        this.planID = plan.id;
        this.planItems = plan.items || [];
      }
    },

    getPlanItem(dayOfWeek, mealType) {
      return this.planItems.find(i => i.day_of_week === dayOfWeek && i.meal_type === mealType) || null;
    },

    async addPlanItem(dayOfWeek, mealType, recipeID) {
      if (!recipeID) return;
      const newItems = [
        ...this.planItems.map(i => ({
          recipe_id: i.recipe_id,
          day_of_week: i.day_of_week,
          meal_type: i.meal_type,
          people_count: i.people_count,
        })),
        { recipe_id: recipeID, day_of_week: dayOfWeek, meal_type: mealType, people_count: 4 },
      ];
      await this.savePlanItems(newItems);
    },

    async removePlanItem(item) {
      const newItems = this.planItems
        .filter(i => i.id !== item.id)
        .map(i => ({
          recipe_id: i.recipe_id,
          day_of_week: i.day_of_week,
          meal_type: i.meal_type,
          people_count: i.people_count,
        }));
      await this.savePlanItems(newItems);
    },

    async updatePeopleCount(item, count) {
      const newItems = this.planItems.map(i => ({
        recipe_id:    i.recipe_id,
        day_of_week:  i.day_of_week,
        meal_type:    i.meal_type,
        people_count: i.id === item.id ? parseInt(count) || 4 : i.people_count,
      }));
      await this.savePlanItems(newItems);
    },

    async savePlanItems(items) {
      const res = await fetch(`/api/weekly-plan/${this.planID}/items`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(items),
      });
      if (res.ok) {
        await this.loadPlanning();
      }
    },

    changeWeek(days) {
      const d = new Date(this.currentWeek);
      d.setDate(d.getDate() + days);
      this.currentWeek = d;
      if (this.page === 'planning') this.loadPlanning();
      if (this.page === 'shopping') this.loadShopping();
    },

    // ── Shopping ────────────────────────────────────────────────────────

    async loadShopping() {
      const week = fmtDate(this.currentWeek);
      const res = await fetch(`/api/shopping-list?week=${week}`);
      if (res.ok) {
        const data = await res.json();
        this.shoppingList = data.items || [];
      }
    },

    async toggleCheck(item, checked) {
      item.checked = checked;
      await fetch('/api/shopping-list/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          week:       fmtDate(this.currentWeek),
          ingredient: item.ingredient,
          checked,
        }),
      });
    },

    printShopping() {
      window.print();
    },

    // ── Import / Web Share Target ────────────────────────────────────────

    async importFromUrl() {
      const url = this.importUrl.trim();
      if (!url) return;
      this.importUrl = '';
      await this.importShared({ url });
    },

    async importShared({ url, text, imageUrl }) {
      this.importing = true;
      this.importError = '';
      const body = {};
      if (url)      body.url       = url;
      if (text)     body.text      = text;
      if (imageUrl) body.image_url = imageUrl;
      try {
        const res = await fetch('/api/import', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
        this.importing = false;
        if (res.ok) {
          this.importSuccess = true;
          await this.loadRecipes();
          this.page = 'recipes';
          setTimeout(() => { this.importSuccess = false; }, 5000);
        } else {
          const err = await res.json();
          this.importError = err.error || 'Erreur lors de l\'import';
        }
      } catch (_) {
        this.importing = false;
        this.importError = 'Erreur réseau lors de l\'import';
      }
    },

    // ── Pantry ──────────────────────────────────────────────────────────

    async loadPantry() {
      this.loadingPantry = true;
      const res = await fetch('/api/pantry');
      if (res.ok) this.pantry = await res.json();
      this.loadingPantry = false;
    },

    openPantryForm(item) {
      this.editingPantryItem = item || null;
      this.pantryForm = item
        ? { ingredient: item.ingredient, quantity: item.quantity, unit: item.unit }
        : { ingredient: '', quantity: 0, unit: '' };
      this.showPantryForm = true;
    },

    async savePantryItem() {
      this.savingPantry = true;
      const body = {
        id: this.editingPantryItem ? this.editingPantryItem.id : '',
        ingredient: this.pantryForm.ingredient,
        quantity: this.pantryForm.quantity,
        unit: this.pantryForm.unit,
      };
      const res = await fetch('/api/pantry', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      this.savingPantry = false;
      if (res.ok) {
        this.showPantryForm = false;
        await this.loadPantry();
      } else {
        const err = await res.json();
        alert('Erreur : ' + (err.error || 'inconnue'));
      }
    },

    async deletePantryItem(id) {
      if (!confirm('Supprimer cet ingrédient du garde-manger ?')) return;
      const res = await fetch(`/api/pantry/${id}`, { method: 'DELETE' });
      if (res.ok) await this.loadPantry();
    },

    // ── Helpers ─────────────────────────────────────────────────────────

    formatWeekLabel(date) {
      if (!date) return '';
      const d = new Date(date);
      const end = new Date(d);
      end.setDate(end.getDate() + 6);
      return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
        + ' – ' + end.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' });
    },

    formatQty(qty) {
      if (qty === Math.floor(qty)) return qty.toString();
      return qty.toFixed(1);
    },
  };
}

// ── Utilities ──────────────────────────────────────────────────────────────

function emptyForm() {
  return {
    title: '',
    description: '',
    instructions: '',
    servings: 4,
    tags: [],
    source_url: '',
    ingredients: [],
  };
}

function mondayOf(date) {
  const d = new Date(date);
  const day = d.getDay(); // 0=Sun
  const diff = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + diff);
  d.setHours(0, 0, 0, 0);
  return d;
}

function fmtDate(date) {
  const d = new Date(date);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}
