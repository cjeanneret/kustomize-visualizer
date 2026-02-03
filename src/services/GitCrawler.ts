import yaml from 'js-yaml';
import type { KustomizeNode, KustomizationYaml } from '../types/kustomize.types';

export class GitCrawler {
    private nodeCounter = 0;
    private githubToken?: string;
    private gitlabToken?: string;

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

    async scanRemoteRepository(repoUrl: string): Promise<KustomizeNode[]> {
        console.log(`\n📡 Scan du repository distant: ${repoUrl}`);

        const { type, owner, repo, branch, basePath } = this.parseRepoUrl(repoUrl);

        console.log(`  Type: ${type}`);
        console.log(`  Owner: ${owner}`);
        console.log(`  Repo: ${repo}`);
        console.log(`  Branch: ${branch || 'default'}`);
        console.log(`  Base path: ${basePath || '/'}`);

        if (type === 'github') {
            return this.scanGitHubRepo(owner, repo, branch, basePath);
        } else if (type === 'gitlab') {
            return this.scanGitLabRepo(owner, repo, branch, basePath);
        } else {
            throw new Error(`Type de repository non supporté: ${type}`);
        }
    }

    private async scanGitHubRepo(
        owner: string,
        repo: string,
        branch?: string,
        basePath?: string
    ): Promise<KustomizeNode[]> {
        console.log('\n🔍 Scan GitHub en cours...');

        const actualBranch = branch || 'main';
        const searchPath = basePath || '';
        const nodes: KustomizeNode[] = [];

        // Headers avec token si disponible
        const headers: Record<string, string> = {
            'Accept': 'application/vnd.github.v3+json'
        };

        if (this.githubToken) {
            headers['Authorization'] = `Bearer ${this.githubToken}`;
            console.log('  🔑 Utilisation du token GitHub');
        } else {
            console.log('  ⚠️ Pas de token - limite: 60 req/h');
        }

        try {
            // Utiliser l'API Tree pour lister TOUS les fichiers récursivement
            const treeUrl = `https://api.github.com/repos/${owner}/${repo}/git/trees/${actualBranch}?recursive=1`;
                console.log(`  📡 Requête Tree API: ${treeUrl}`);

            const treeResponse = await fetch(treeUrl, { headers });

            if (!treeResponse.ok) {
                if (treeResponse.status === 403) {
                    const rateLimitReset = treeResponse.headers.get('X-RateLimit-Reset');
                    const resetDate = rateLimitReset
                        ? new Date(parseInt(rateLimitReset) * 1000).toLocaleTimeString()
                        : 'inconnu';
                        throw new Error(
                            `Rate limit GitHub atteint. Réinitialisation à ${resetDate}. ` +
                                `Ajoutez un token GitHub pour augmenter la limite à 5000 req/h.`
                        );
                }
                throw new Error(`Erreur GitHub API: ${treeResponse.status} ${treeResponse.statusText}`);
            }

            const treeData = await treeResponse.json();

            // Vérifier si l'arbre est tronqué
            if (treeData.truncated) {
                console.warn(`  ⚠️ ATTENTION : L'arbre est tronqué ! (${treeData.tree.length} entrées)`);
                console.warn(`  ⚠️ Certains fichiers peuvent manquer.`);
            }

            console.log(`  📊 Total d'entrées dans l'arbre: ${treeData.tree.length}`);

            // Filtrer pour ne garder que les kustomization.yaml
            const kustomizationFiles = treeData.tree.filter((item: any) =>
                                                            item.type === 'blob' &&
                                                                (item.path.endsWith('/kustomization.yaml') || item.path === 'kustomization.yaml')
                                                           );

                                                           console.log(`  ✓ ${kustomizationFiles.length} fichier(s) kustomization.yaml trouvé(s)`);

                                                           // Traiter chaque fichier trouvé
                                                           for (const item of kustomizationFiles) {
                                                               const filePath = item.path;

                                                               // Filtrer par basePath si spécifié
                                                               if (searchPath && !filePath.startsWith(searchPath)) {
                                                                   continue;
                                                               }

                                                               console.log(`\n  📄 Traitement: ${filePath}`);

                                                               // Télécharger le contenu via l'API Contents
                                                               const contentUrl = `https://api.github.com/repos/${owner}/${repo}/contents/${filePath}?ref=${actualBranch}`;
                                                                   const contentResponse = await fetch(contentUrl, { headers });

                                                               if (!contentResponse.ok) {
                                                                   console.warn(`    ⚠️ Erreur: ${contentResponse.status}`);
                                                                   continue;
                                                               }

                                                               const contentData = await contentResponse.json();

                                                               // Décoder le base64 (compatible navigateur)
                                                               const base64Content = contentData.content.replace(/\n/g, '');
                                                               const binaryString = atob(base64Content);
                                                               const bytes = new Uint8Array(binaryString.length);
                                                               for (let i = 0; i < binaryString.length; i++) {
                                                                   bytes[i] = binaryString.charCodeAt(i);
                                                               }
                                                               const content = new TextDecoder('utf-8').decode(bytes);

                                                               try {
                                                                   const kustomization = yaml.load(content) as KustomizationYaml;

                                                                   // Extraire le chemin du dossier
                                                                   const dirPath = filePath.replace(/\/kustomization\.yaml$/, '') || '.';

                                                                   const node = this.createNode(dirPath, kustomization, false);
                                                                   nodes.push(node);

                                                                   console.log(`    ✓ Nœud créé: ${node.path} (type: ${node.type})`);
                                                               } catch (err) {
                                                                   console.warn(`    ⚠️ Erreur parsing YAML: ${err}`);
                                                               }
                                                           }

                                                           console.log(`\n✅ Scan GitHub terminé: ${nodes.length} nœud(s)`);
                                                           return nodes;

        } catch (error) {
            console.error('❌ Erreur lors du scan GitHub:', error);
            throw error;
        }
    }



    private async scanGitLabRepo(
        owner: string,
        repo: string,
        branch?: string,
        basePath?: string
    ): Promise<KustomizeNode[]> {
        console.log('\n🔍 Scan GitLab en cours...');

        const actualBranch = branch || 'main';
        const searchPath = basePath || '';
        const nodes: KustomizeNode[] = [];

        // Headers avec token si disponible
        const headers: Record<string, string> = {};

        if (this.gitlabToken) {
            headers['PRIVATE-TOKEN'] = this.gitlabToken;
            console.log('  🔑 Utilisation du token GitLab');
        } else {
            console.log('  ⚠️ Pas de token - accès limité');
        }

        try {
            const projectPath = encodeURIComponent(`${owner}/${repo}`);
            const treeUrl = `https://gitlab.com/api/v4/projects/${projectPath}/repository/tree?ref=${actualBranch}&recursive=true&per_page=100`;

                console.log(`  📡 Requête: ${treeUrl}`);

            const treeResponse = await fetch(treeUrl, { headers });

            if (!treeResponse.ok) {
                if (treeResponse.status === 401) {
                    throw new Error('GitLab: Token invalide ou manquant pour ce projet privé');
                }
                throw new Error(`Erreur GitLab API: ${treeResponse.status} ${treeResponse.statusText}`);
            }

            const tree = await treeResponse.json();
            const kustomizationFiles = tree.filter((item: any) =>
                                                   item.type === 'blob' && item.name === 'kustomization.yaml'
                                                  );

                                                  console.log(`  ✓ ${kustomizationFiles.length} fichier(s) trouvé(s)`);

                                                  for (const file of kustomizationFiles) {
                                                      const filePath = file.path;

                                                      if (searchPath && !filePath.startsWith(searchPath)) {
                                                          continue;
                                                      }

                                                      console.log(`\n  📄 Traitement: ${filePath}`);

                                                      const fileUrl = `https://gitlab.com/api/v4/projects/${projectPath}/repository/files/${encodeURIComponent(filePath)}/raw?ref=${actualBranch}`;
                                                          const fileResponse = await fetch(fileUrl, { headers });

                                                      if (!fileResponse.ok) {
                                                          console.warn(`    ⚠️ Erreur: ${fileResponse.status}`);
                                                          continue;
                                                      }

                                                      const content = await fileResponse.text();

                                                      try {
                                                          const kustomization = yaml.load(content) as KustomizationYaml;
                                                          const dirPath = filePath.replace(/\/kustomization\.yaml$/, '') || '.';

                                                          const node = this.createNode(dirPath, kustomization, false);
                                                          nodes.push(node);

                                                          console.log(`    ✓ Nœud créé: ${node.path} (type: ${node.type})`);
                                                      } catch (err) {
                                                          console.warn(`    ⚠️ Erreur parsing YAML: ${err}`);
                                                      }
                                                  }

                                                  console.log(`\n✅ Scan GitLab terminé: ${nodes.length} nœud(s)`);
                                                  return nodes;

        } catch (error) {
            console.error('❌ Erreur lors du scan GitLab:', error);
            throw error;
        }
    }

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

                        const node = this.createNode(relativePath, kustomization, false);
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

    private createNode(
        path: string,
        kustomization: KustomizationYaml,
        isRemote: boolean
    ): KustomizeNode {
        // Type par défaut : resource
        // Sera corrigé plus tard selon comment il est référencé
        return {
            id: `node-${this.nodeCounter++}`,
            path,
            type: 'resource',  // Par défaut
            kustomizationContent: kustomization,
            isRemote,
            loaded: true
        };
    }

    private parseRepoUrl(url: string): {
        type: 'github' | 'gitlab';
        owner: string;
        repo: string;
        branch?: string;
        basePath?: string;
    } {
        // GitHub
        const githubMatch = url.match(
            /github\.com\/([^\/]+)\/([^\/]+)(?:\/tree\/([^\/]+)(?:\/(.+))?)?/
        );

        if (githubMatch) {
            return {
                type: 'github',
                owner: githubMatch[1],
                repo: githubMatch[2].replace(/\.git$/, ''),
                branch: githubMatch[3],
                basePath: githubMatch[4]
            };
        }

        // GitLab
        const gitlabMatch = url.match(
            /gitlab\.com\/([^\/]+)\/([^\/]+)(?:\/-\/tree\/([^\/]+)(?:\/(.+))?)?/
        );

        if (gitlabMatch) {
            return {
                type: 'gitlab',
                owner: gitlabMatch[1],
                repo: gitlabMatch[2].replace(/\.git$/, ''),
                branch: gitlabMatch[3],
                basePath: gitlabMatch[4]
            };
        }

        throw new Error('URL non reconnue. Format attendu: https://github.com/owner/repo ou https://gitlab.com/owner/repo');
    }
}

