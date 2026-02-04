/**
 * Service de gestion sécurisée des tokens API
 * Utilise Web Crypto API pour chiffrer les tokens dans localStorage
 */
export class TokenManager {
    private static readonly STORAGE_KEY_GITHUB = 'kv_github_token_enc';
    private static readonly STORAGE_KEY_GITLAB = 'kv_gitlab_token_enc';
    private static readonly STORAGE_KEY_SALT = 'kv_token_salt';
    
    // Clé de base (dans un vrai cas, ce serait plus complexe)
    // Note: Cette approche offre une obfuscation, pas une vraie sécurité
    private static readonly ENCRYPTION_BASE = 'kustomize-visualizer-v1';

    /**
     * Initialiser le salt (généré une seule fois par navigateur)
     */
    private static async getOrCreateSalt(): Promise<Uint8Array> {
        let saltHex = localStorage.getItem(this.STORAGE_KEY_SALT);
        
        if (!saltHex) {
            // Générer un nouveau salt aléatoire
            const salt = crypto.getRandomValues(new Uint8Array(16));
            saltHex = Array.from(salt)
                .map(b => b.toString(16).padStart(2, '0'))
                .join('');
            localStorage.setItem(this.STORAGE_KEY_SALT, saltHex);
        }
        
        // Convertir hex en Uint8Array
        const matches = saltHex.match(/.{1,2}/g) || [];
        return new Uint8Array(matches.map(byte => parseInt(byte, 16)));
    }

    /**
     * Dériver une clé de chiffrement depuis la base + salt
     */
    private static async deriveKey(salt: Uint8Array): Promise<CryptoKey> {
        const encoder = new TextEncoder();
        const keyMaterial = await crypto.subtle.importKey(
            'raw',
            encoder.encode(this.ENCRYPTION_BASE),
            { name: 'PBKDF2' },
            false,
            ['deriveBits', 'deriveKey']
        );

        return crypto.subtle.deriveKey(
            {
                name: 'PBKDF2',
                salt: salt,
                iterations: 100000,
                hash: 'SHA-256'
            },
            keyMaterial,
            { name: 'AES-GCM', length: 256 },
            false,
            ['encrypt', 'decrypt']
        );
    }

    /**
     * Chiffrer un token
     */
    private static async encryptToken(token: string): Promise<string> {
        const salt = await this.getOrCreateSalt();
        const key = await this.deriveKey(salt);
        
        const encoder = new TextEncoder();
        const data = encoder.encode(token);
        
        // IV aléatoire pour AES-GCM
        const iv = crypto.getRandomValues(new Uint8Array(12));
        
        const encrypted = await crypto.subtle.encrypt(
            { name: 'AES-GCM', iv: iv },
            key,
            data
        );
        
        // Combiner IV + données chiffrées en hex
        const encryptedArray = new Uint8Array(encrypted);
        const combined = new Uint8Array(iv.length + encryptedArray.length);
        combined.set(iv);
        combined.set(encryptedArray, iv.length);
        
        return Array.from(combined)
            .map(b => b.toString(16).padStart(2, '0'))
            .join('');
    }

    /**
     * Déchiffrer un token
     */
    private static async decryptToken(encryptedHex: string): Promise<string> {
        try {
            const salt = await this.getOrCreateSalt();
            const key = await this.deriveKey(salt);
            
            // Convertir hex en bytes
            const matches = encryptedHex.match(/.{1,2}/g) || [];
            const combined = new Uint8Array(matches.map(byte => parseInt(byte, 16)));
            
            // Séparer IV et données
            const iv = combined.slice(0, 12);
            const data = combined.slice(12);
            
            const decrypted = await crypto.subtle.decrypt(
                { name: 'AES-GCM', iv: iv },
                key,
                data
            );
            
            const decoder = new TextDecoder();
            return decoder.decode(decrypted);
        } catch (error) {
            console.error('Erreur de déchiffrement:', error);
            throw new Error('Token corrompu ou invalide');
        }
    }

    /**
     * Sauvegarder le token GitHub
     */
    static async saveGitHubToken(token: string): Promise<void> {
        if (!token || token.trim() === '') {
            this.clearGitHubToken();
            return;
        }
        
        const encrypted = await this.encryptToken(token.trim());
        localStorage.setItem(this.STORAGE_KEY_GITHUB, encrypted);
        console.log('✓ Token GitHub sauvegardé (chiffré)');
    }

    /**
     * Récupérer le token GitHub
     */
    static async getGitHubToken(): Promise<string | null> {
        const encrypted = localStorage.getItem(this.STORAGE_KEY_GITHUB);
        if (!encrypted) {
            return null;
        }
        
        try {
            return await this.decryptToken(encrypted);
        } catch (error) {
            console.error('Erreur lors de la récupération du token GitHub');
            this.clearGitHubToken();
            return null;
        }
    }

    /**
     * Supprimer le token GitHub
     */
    static clearGitHubToken(): void {
        localStorage.removeItem(this.STORAGE_KEY_GITHUB);
        console.log('✓ Token GitHub supprimé');
    }

    /**
     * Sauvegarder le token GitLab
     */
    static async saveGitLabToken(token: string): Promise<void> {
        if (!token || token.trim() === '') {
            this.clearGitLabToken();
            return;
        }
        
        const encrypted = await this.encryptToken(token.trim());
        localStorage.setItem(this.STORAGE_KEY_GITLAB, encrypted);
        console.log('✓ Token GitLab sauvegardé (chiffré)');
    }

    /**
     * Récupérer le token GitLab
     */
    static async getGitLabToken(): Promise<string | null> {
        const encrypted = localStorage.getItem(this.STORAGE_KEY_GITLAB);
        if (!encrypted) {
            return null;
        }
        
        try {
            return await this.decryptToken(encrypted);
        } catch (error) {
            console.error('Erreur lors de la récupération du token GitLab');
            this.clearGitLabToken();
            return null;
        }
    }

    /**
     * Supprimer le token GitLab
     */
    static clearGitLabToken(): void {
        localStorage.removeItem(this.STORAGE_KEY_GITLAB);
        console.log('✓ Token GitLab supprimé');
    }

    /**
     * Vérifier si des tokens sont stockés
     */
    static hasGitHubToken(): boolean {
        return localStorage.getItem(this.STORAGE_KEY_GITHUB) !== null;
    }

    static hasGitLabToken(): boolean {
        return localStorage.getItem(this.STORAGE_KEY_GITLAB) !== null;
    }

    /**
     * Tout effacer (tokens + salt)
     */
    static clearAll(): void {
        localStorage.removeItem(this.STORAGE_KEY_GITHUB);
        localStorage.removeItem(this.STORAGE_KEY_GITLAB);
        localStorage.removeItem(this.STORAGE_KEY_SALT);
        console.log('✓ Tous les tokens et données supprimés');
    }
}
