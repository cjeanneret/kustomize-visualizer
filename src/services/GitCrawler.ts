import yaml from 'js-yaml';
import type { KustomizeNode, KustomizationYaml } from '../types/kustomize.types';

interface GitUrlComponents {
    provider: 'github' | 'gitlab';
    host: string;  // Pour supporter gitlab.com et instances internes
    owner: string;
    repo: string;
    ref?: string;
    path: string;
}

export class GitCrawler {
    private nodeCounter = 0;
    private githubToken?: string;
    private gitlabToken?: string;
    private visited = new Set<string>();

    /**
     * Définir le token GitHub
     */
    setGitHubToken(token: string): void {
        this.githubToken = token;
    }

    /**
     * Définir le token GitLab
     */
    setGitLabToken(token: string): void {
        this.gitlabToken = token;
    }

    /**
     * Point d'entrée principal : crawl récursif depuis une URL d'overlay
     */
    async crawlFromOverlay(overlayUrl: string): Promise<KustomizeNode[]> {
        console.log(`\n🚀 Démarrage du crawl depuis: ${overlayUrl}`);

        this.nodeCounter = 0;
        this.visited.clear();

        const nodes: KustomizeNode[] = [];

        try {
            await this.crawlKustomization(overlayUrl, nodes, null);
            console.log(`\n✅ Crawl terminé: ${nodes.length} nœud(s) découvert(s)`);
            return nodes;
        } catch (error) {
            console.error('❌ Erreur lors du crawl:', error);
            throw error;
        }
    }

    /**
     * Crawl récursif d'un kustomization.yaml
     */
    private async crawlKustomization(
        url: string,
        nodes: KustomizeNode[],
        referenceType: 'resource' | 'component' | null
    ): Promise<KustomizeNode> {
        // Normaliser l'URL pour détecter les doublons
        const normalizedUrl = this.normalizeUrl(url);

        // Vérifier si déjà visité
        if (this.visited.has(normalizedUrl)) {
            console.log(`  ⏭️ Déjà visité: ${normalizedUrl}`);
            // Trouver le nœud existant
            const existingNode = nodes.find(n => n.remoteUrl === normalizedUrl);
            if (existingNode) {
                return existingNode;
            }
        }

        this.visited.add(normalizedUrl);
        console.log(`\n🔍 Crawl: ${normalizedUrl}`);

        // Télécharger et parser le kustomization.yaml
        let kustomization: KustomizationYaml;
        let kustomizationUrl: string;

        try {
            kustomizationUrl = this.ensureKustomizationYaml(normalizedUrl);
            const content = await this.fetchFileContent(kustomizationUrl);
            kustomization = yaml.load(content) as KustomizationYaml;
            console.log(`  ✓ kustomization.yaml chargé`);
        } catch (error) {
            console.warn(`  ⚠️ Impossible de charger kustomization.yaml:`, error);
            throw error;
        }

        // Créer le nœud
        const node = this.createNode(normalizedUrl, kustomization, referenceType);
        nodes.push(node);
        console.log(`  ✓ Nœud créé: ${node.id} (type: ${node.type})`);

        // Traiter les resources
        if (kustomization.resources && kustomization.resources.length > 0) {
            console.log(`  📦 Traitement de ${kustomization.resources.length} resource(s)...`);
            for (const resource of kustomization.resources) {
                await this.processResource(resource, normalizedUrl, nodes);
            }
        }

        // Traiter les bases (déprécié, traité comme resources)
        if (kustomization.bases && kustomization.bases.length > 0) {
            console.log(`  📦 Traitement de ${kustomization.bases.length} base(s) [déprécié]...`);
            for (const base of kustomization.bases) {
                await this.processResource(base, normalizedUrl, nodes);
            }
        }

        // Traiter les components
        if (kustomization.components && kustomization.components.length > 0) {
            console.log(`  🧩 Traitement de ${kustomization.components.length} component(s)...`);
            for (const component of kustomization.components) {
                await this.processComponent(component, normalizedUrl, nodes);
            }
        }

        return node;
    }

    /**
     * Traiter une resource
     */
    private async processResource(
        resource: string,
        parentUrl: string,
        nodes: KustomizeNode[]
    ): Promise<void> {
        console.log(`    📄 Resource: ${resource}`);

        // SI c'est un fichier YAML simple, IGNORER
        if (this.isYamlFile(resource)) {
            console.log(`      ⏭️ Ignoré (fichier YAML simple)`);
            return;
        }

        // Résoudre l'URL complète
        const resolvedUrl = this.resolveUrl(parentUrl, resource);
        console.log(`      → Résolu: ${resolvedUrl}`);

        // Vérifier si un kustomization.yaml existe
        try {
            await this.crawlKustomization(resolvedUrl, nodes, 'resource');
        } catch (error) {
            console.warn(`      ⚠️ Pas de kustomization.yaml trouvé (ignoré)`);
        }
    }

    /**
     * Traiter un component
     */
    private async processComponent(
        component: string,
        parentUrl: string,
        nodes: KustomizeNode[]
    ): Promise<void> {
        console.log(`    🧩 Component: ${component}`);

        // Résoudre l'URL complète
        const resolvedUrl = this.resolveUrl(parentUrl, component);
        console.log(`      → Résolu: ${resolvedUrl}`);

        // Les components DOIVENT avoir un kustomization.yaml
        try {
            await this.crawlKustomization(resolvedUrl, nodes, 'component');
        } catch (error) {
            console.error(`      ❌ Erreur: component sans kustomization.yaml`);
            throw error;
        }
    }

    /**
     * Créer un nœud
     */
    private createNode(
        url: string,
        kustomization: KustomizationYaml,
        referenceType: 'resource' | 'component' | null
    ): KustomizeNode {
        const components = this.parseGitUrl(url);
        const displayPath = components.path || '.';

        // Le type est déterminé par comment il est référencé
        // Si c'est le nœud racine (referenceType = null), on considère comme resource
        const type = referenceType || 'resource';

        return {
            id: `node-${this.nodeCounter++}`,
            path: displayPath,
            type,
            kustomizationContent: kustomization,
            isRemote: true,
            remoteUrl: url,
            loaded: true
        };
    }

    /**
     * Vérifier si c'est un fichier YAML simple
     */
    private isYamlFile(path: string): boolean {
        const lower = path.toLowerCase();
        return (lower.endsWith('.yaml') || lower.endsWith('.yml')) &&
               !lower.endsWith('kustomization.yaml') &&
               !lower.endsWith('kustomization.yml');
    }

    /**
     * Vérifier si c'est une URL complète
     */
    private isFullUrl(path: string): boolean {
        return path.startsWith('http://') || path.startsWith('https://');
    }

    /**
     * Résoudre une URL (équivalent os.path.join)
     */
    private resolveUrl(baseUrl: string, relativePath: string): string {
        // SI c'est déjà une URL complète, la retourner telle quelle
        if (this.isFullUrl(relativePath)) {
            return relativePath;
        }

        // Parser l'URL de base
        const components = this.parseGitUrl(baseUrl);

        // Résoudre le chemin (comme os.path.join avec support de ..)
        const resolvedPath = this.joinPaths(components.path, relativePath);

        // Reconstruire l'URL
        return this.buildGitUrl(
            components.provider,
            components.host,
            components.owner,
            components.repo,
            components.ref,
            resolvedPath
        );
    }

    /**
     * Joindre des chemins (équivalent os.path.join avec support de ..)
     */
    private joinPaths(basePath: string, relativePath: string): string {
        // Normaliser
        const baseParts = basePath.split('/').filter(p => p && p !== '.');
        const relativeParts = relativePath.split('/').filter(p => p && p !== '.');

        // Appliquer les ".." pour remonter
        for (const part of relativeParts) {
            if (part === '..') {
                baseParts.pop();
            } else {
                baseParts.push(part);
            }
        }

        return baseParts.join('/') || '.';
    }

    /**
     * Normaliser une URL (retirer /tree/, gérer ?ref=)
     */
    private normalizeUrl(url: string): string {
        // Retirer /tree/ (GitHub) et /-/tree/ (GitLab)
        let normalized = url.replace(/\/tree\/[^\/]+/, '').replace(/\/-\/tree\/[^\/]+/, '');

        // Retirer le trailing slash
        normalized = normalized.replace(/\/$/, '');

        return normalized;
    }

    /**
     * S'assurer que l'URL pointe vers kustomization.yaml
     */
    private ensureKustomizationYaml(url: string): string {
        if (url.endsWith('kustomization.yaml') || url.endsWith('kustomization.yml')) {
            return url;
        }

        // Ajouter /kustomization.yaml
        return `${url}/kustomization.yaml`;
    }

    /**
     * Parser une URL Git (GitHub ou GitLab)
     */
    private parseGitUrl(url: string): GitUrlComponents {
        // Séparer l'URL de base et les paramètres de requête
        const [baseUrl, queryString] = url.split('?');

        // Extraire ?ref=VALUE (branch, tag ou hash)
        const refMatch = queryString?.match(/ref=([^&]+)/);
        const ref = refMatch ? decodeURIComponent(refMatch[1]) : undefined;

        // GitHub: https://github.com/owner/repo/path ou https://github.com/owner/repo?ref=branch
        const githubMatch = baseUrl.match(/https?:\/\/(github\.com)\/([^\/]+)\/([^\/]+)(?:\/(.*))?/);

        if (githubMatch) {
            return {
                provider: 'github',
                host: githubMatch[1],
                owner: githubMatch[2],
                repo: githubMatch[3].replace(/\.git$/, ''),
                ref,
                path: githubMatch[4] || ''
            };
        }

        // GitLab: https://gitlab.com/owner/repo/path ou instances internes
        // Note: on détecte "gitlab" dans le hostname
        const gitlabMatch = baseUrl.match(/https?:\/\/([^\/]*gitlab[^\/]*)\/([^\/]+)\/([^\/]+)(?:\/(.*))?/);

        if (gitlabMatch) {
            return {
                provider: 'gitlab',
                host: gitlabMatch[1],
                owner: gitlabMatch[2],
                repo: gitlabMatch[3].replace(/\.git$/, ''),
                ref,
                path: gitlabMatch[4] || ''
            };
        }

        throw new Error(`URL non reconnue: ${url}. Formats supportés: GitHub et GitLab`);
    }

    /**
     * Construire une URL Git
     */
    private buildGitUrl(
        provider: 'github' | 'gitlab',
        host: string,
        owner: string,
        repo: string,
        ref: string | undefined,
        path: string
    ): string {
        const pathPart = path ? `/${path}` : '';
        const baseUrl = `https://${host}/${owner}/${repo}${pathPart}`;

        return ref ? `${baseUrl}?ref=${encodeURIComponent(ref)}` : baseUrl;
    }

    /**
     * Télécharger le contenu d'un fichier
     */
    private async fetchFileContent(url: string): Promise<string> {
        const components = this.parseGitUrl(url);

        if (components.provider === 'github') {
            return this.fetchGitHubFile(components);
        } else if (components.provider === 'gitlab') {
            return this.fetchGitLabFile(components);
        }

        throw new Error(`Provider non supporté: ${components.provider}`);
    }

    /**
     * Télécharger depuis GitHub
     */
    private async fetchGitHubFile(components: GitUrlComponents): Promise<string> {
        const ref = components.ref || 'main';
        const apiUrl = `https://api.${components.host}/repos/${components.owner}/${components.repo}/contents/${components.path}?ref=${ref}`;

        const headers: Record<string, string> = {
            'Accept': 'application/vnd.github.v3+json'
        };

        if (this.githubToken) {
            headers['Authorization'] = `Bearer ${this.githubToken}`;
        }

        const response = await fetch(apiUrl, { headers });

        if (!response.ok) {
            if (response.status === 403) {
                const rateLimitReset = response.headers.get('X-RateLimit-Reset');
                const resetDate = rateLimitReset
                    ? new Date(parseInt(rateLimitReset) * 1000).toLocaleTimeString()
                    : 'inconnu';
                throw new Error(
                    `Rate limit GitHub atteint. Réinitialisation à ${resetDate}. ` +
                    `Ajoutez un token GitHub pour augmenter la limite à 5000 req/h.`
                );
            }
            throw new Error(`GitHub API error: ${response.status} ${response.statusText}`);
        }

        const data = await response.json();

        // Décoder le contenu base64
        const base64Content = data.content.replace(/\n/g, '');
        const binaryString = atob(base64Content);
        const bytes = new Uint8Array(binaryString.length);
        for (let i = 0; i < binaryString.length; i++) {
            bytes[i] = binaryString.charCodeAt(i);
        }

        return new TextDecoder('utf-8').decode(bytes);
    }

    /**
     * Télécharger depuis GitLab
     */
    private async fetchGitLabFile(components: GitUrlComponents): Promise<string> {
        const ref = components.ref || 'main';
        const projectPath = encodeURIComponent(`${components.owner}/${components.repo}`);
        const filePath = encodeURIComponent(components.path);
        const apiUrl = `https://${components.host}/api/v4/projects/${projectPath}/repository/files/${filePath}/raw?ref=${ref}`;

        const headers: Record<string, string> = {};

        if (this.gitlabToken) {
            headers['PRIVATE-TOKEN'] = this.gitlabToken;
        }

        const response = await fetch(apiUrl, { headers });

        if (!response.ok) {
            if (response.status === 401) {
                throw new Error('GitLab: Token invalide ou manquant pour ce projet privé');
            }
            throw new Error(`GitLab API error: ${response.status} ${response.statusText}`);
        }

        return await response.text();
    }

    /**
     * Scan local (conservé pour compatibilité)
     */
    async scanLocalDirectory(): Promise<KustomizeNode[]> {
        console.log('\n📁 Scan du répertoire local...');

        if (typeof window === 'undefined' || !window.electron) {
            throw new Error('Le scan local nécessite Electron');
        }

        try {
            const directoryPath = await window.electron.selectDirectory();

            if (!directoryPath) {
                throw new Error('Aucun répertoire sélectionné');
            }

            console.log(`  📂 Répertoire: ${directoryPath}`);

            const files = await window.electron.findKustomizationFiles(directoryPath);
            console.log(`  ✓ ${files.length} fichier(s) trouvé(s)`);

            const nodes: KustomizeNode[] = [];

            for (const filePath of files) {
                console.log(`\n  📄 Traitement: ${filePath}`);

                const content = await window.electron.readFile(filePath);

                try {
                    const kustomization = yaml.load(content) as KustomizationYaml;

                    const relativePath = filePath
                        .replace(directoryPath, '')
                        .replace(/^[\/\\]/, '')
                        .replace(/[\/\\]kustomization\.yaml$/, '')
                        .replace(/\\/g, '/')
                        || '.';

                    const node: KustomizeNode = {
                        id: `node-${this.nodeCounter++}`,
                        path: relativePath,
                        type: 'resource',
                        kustomizationContent: kustomization,
                        isRemote: false,
                        loaded: true
                    };

                    nodes.push(node);

                    console.log(`    ✓ Nœud créé: ${node.path} (type: ${node.type})`);
                } catch (err) {
                    console.warn(`    ⚠️ Erreur parsing YAML: ${err}`);
                }
            }

            console.log(`\n✅ Scan local terminé: ${nodes.length} nœud(s)`);
            return nodes;

        } catch (error) {
            console.error('❌ Erreur lors du scan local:', error);
            throw error;
        }
    }
}
