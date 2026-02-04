import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { LanguageSwitcher } from '../LanguageSwitcher/LanguageSwitcher';
import { TokenManager } from '../../services/TokenManager';
import './CollapsibleSidebar.css';

interface CollapsibleSidebarProps {
  onLoadRepo: (source: string, isLocal: boolean, githubToken?: string, gitlabToken?: string) => Promise<void>;
}

export const CollapsibleSidebar: React.FC<CollapsibleSidebarProps> = ({ onLoadRepo }) => {
  const { t } = useTranslation();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [repoUrl, setRepoUrl] = useState('');
  const [githubToken, setGithubToken] = useState('');
  const [gitlabToken, setGitlabToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastLoaded, setLastLoaded] = useState<string>('');

  // Indicateurs de tokens sauvegardés
  const [hasStoredGithubToken, setHasStoredGithubToken] = useState(false);
  const [hasStoredGitlabToken, setHasStoredGitlabToken] = useState(false);

  // Charger les tokens au démarrage
  useEffect(() => {
    const loadTokens = async () => {
      const gh = await TokenManager.getGitHubToken();
      const gl = await TokenManager.getGitLabToken();

      if (gh) {
        setGithubToken(gh);
        setHasStoredGithubToken(true);
        console.log('✓ Token GitHub chargé depuis le localStorage');
      }
      if (gl) {
        setGitlabToken(gl);
        setHasStoredGitlabToken(true);
        console.log('✓ Token GitLab chargé depuis le localStorage');
      }
    };

    loadTokens();
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!repoUrl.trim()) return;

    setLoading(true);
    setError(null);

    try {
      await onLoadRepo(repoUrl, false, githubToken || undefined, gitlabToken || undefined);
      setLastLoaded(repoUrl);
    } catch (err) {
      const message = err instanceof Error ? err.message : t('errors.unknownError');
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  const handleLoadLocal = async () => {
    setLoading(true);
    setError(null);

    try {
      await onLoadRepo('', true);
      setLastLoaded(t('sidebar.selectLocal'));
    } catch (err) {
      const message = err instanceof Error ? err.message : t('errors.unknownError');
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  // Sauvegarder le token GitHub
  const handleSaveGithubToken = async () => {
    try {
      await TokenManager.saveGitHubToken(githubToken);
      setHasStoredGithubToken(githubToken.trim() !== '');
      alert(t('sidebar.tokens.savedSuccess', { provider: 'GitHub' }));
    } catch (err) {
      alert(t('sidebar.tokens.saveError'));
    }
  };

  // Sauvegarder le token GitLab
  const handleSaveGitlabToken = async () => {
    try {
      await TokenManager.saveGitLabToken(gitlabToken);
      setHasStoredGitlabToken(gitlabToken.trim() !== '');
      alert(t('sidebar.tokens.savedSuccess', { provider: 'GitLab' }));
    } catch (err) {
      alert(t('sidebar.tokens.saveError'));
    }
  };

  // Effacer le token GitHub
  const handleClearGithubToken = () => {
    TokenManager.clearGitHubToken();
    setGithubToken('');
    setHasStoredGithubToken(false);
  };

  // Effacer le token GitLab
  const handleClearGitlabToken = () => {
    TokenManager.clearGitLabToken();
    setGitlabToken('');
    setHasStoredGitlabToken(false);
  };

  // Effacer tous les tokens
  const handleClearAllTokens = () => {
    if (confirm(t('sidebar.tokens.confirmClearAll'))) {
      TokenManager.clearAll();
      setGithubToken('');
      setGitlabToken('');
      setHasStoredGithubToken(false);
      setHasStoredGitlabToken(false);
      alert(t('sidebar.tokens.clearedSuccess'));
    }
  };

  return (
    <div className={`collapsible-sidebar left ${isCollapsed ? 'collapsed' : ''}`}>
      <button
        className="collapse-toggle"
        onClick={() => setIsCollapsed(!isCollapsed)}
        title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      >
        {isCollapsed ? '▶' : '◀'}
      </button>

      {!isCollapsed && (
        <div className="sidebar-content">
          <h1>{t('sidebar.title')}</h1>

          <LanguageSwitcher />

          <form onSubmit={handleSubmit} className="load-form">
            <label htmlFor="repo-url">{t('sidebar.repoUrlLabel')}</label>
            <input
              id="repo-url"
              type="text"
              value={repoUrl}
              onChange={(e) => setRepoUrl(e.target.value)}
              placeholder={t('sidebar.repoUrlPlaceholder')}
              disabled={loading}
            />

            <details className="token-section" open={!hasStoredGithubToken && !hasStoredGitlabToken}>
              <summary>
                🔑 {t('sidebar.tokens.title')}
                {(hasStoredGithubToken || hasStoredGitlabToken) && (
                  <span className="token-indicator"> ● {t('sidebar.tokens.saved')}</span>
                )}
              </summary>

              <div className="token-field">
                <label htmlFor="github-token">
                  GitHub Token
                  {hasStoredGithubToken && <span className="stored-badge">💾 {t('sidebar.tokens.stored')}</span>}
                  <a href="https://github.com/settings/tokens" target="_blank" rel="noopener noreferrer" className="help-link">
                    ?
                  </a>
                </label>
                <div className="token-input-group">
                  <input
                    id="github-token"
                    type="password"
                    value={githubToken}
                    onChange={(e) => setGithubToken(e.target.value)}
                    placeholder="ghp_..."
                    disabled={loading}
                  />
                  <button
                    type="button"
                    onClick={handleSaveGithubToken}
                    disabled={!githubToken.trim()}
                    className="save-token-btn"
                    title={t('sidebar.tokens.saveButton')}
                  >
                    💾
                  </button>
                  {hasStoredGithubToken && (
                    <button
                      type="button"
                      onClick={handleClearGithubToken}
                      className="clear-token-btn"
                      title={t('sidebar.tokens.clearButton')}
                    >
                      🗑️
                    </button>
                  )}
                </div>
              </div>

              <div className="token-field">
                <label htmlFor="gitlab-token">
                  GitLab Token
                  {hasStoredGitlabToken && <span className="stored-badge">💾 {t('sidebar.tokens.stored')}</span>}
                  <a href="https://gitlab.com/-/profile/personal_access_tokens" target="_blank" rel="noopener noreferrer" className="help-link">
                    ?
                  </a>
                </label>
                <div className="token-input-group">
                  <input
                    id="gitlab-token"
                    type="password"
                    value={gitlabToken}
                    onChange={(e) => setGitlabToken(e.target.value)}
                    placeholder="glpat-..."
                    disabled={loading}
                  />
                  <button
                    type="button"
                    onClick={handleSaveGitlabToken}
                    disabled={!gitlabToken.trim()}
                    className="save-token-btn"
                    title={t('sidebar.tokens.saveButton')}
                  >
                    💾
                  </button>
                  {hasStoredGitlabToken && (
                    <button
                      type="button"
                      onClick={handleClearGitlabToken}
                      className="clear-token-btn"
                      title={t('sidebar.tokens.clearButton')}
                    >
                      🗑️
                    </button>
                  )}
                </div>
              </div>

              {(hasStoredGithubToken || hasStoredGitlabToken) && (
                <button
                  type="button"
                  onClick={handleClearAllTokens}
                  className="clear-all-tokens-btn"
                >
                  🗑️ {t('sidebar.tokens.clearAll')}
                </button>
              )}

              <small className="token-help">
                🔒 {t('sidebar.tokens.encryptedInfo')}
                <br />
                {t('sidebar.tokens.rateLimitHelp')}
              </small>
            </details>

            <button type="submit" disabled={loading || !repoUrl.trim()}>
              {loading ? `🔄 ${t('sidebar.loading')}` : `🌐 ${t('sidebar.loadRemote')}`}
            </button>
          </form>

          <div className="separator">
            <span>{t('sidebar.orSeparator')}</span>
          </div>

          <button
            onClick={handleLoadLocal}
            disabled={loading}
            className="load-button local-button"
          >
            {loading ? `🔄 ${t('sidebar.loading')}` : `📁 ${t('sidebar.selectLocal')}`}
          </button>

          {lastLoaded && !error && (
            <div className="success-message">
              ✓ {t('sidebar.loadedSuccess', { source: lastLoaded })}
            </div>
          )}

          {error && (
            <div className="error-message">
              <strong>{t('sidebar.errorPrefix')}</strong> {error}
            </div>
          )}

          <div className="info-section">
            <h3>{t('sidebar.supportedSources')}</h3>
            <ul>
              <li><strong>GitHub</strong></li>
              <li><strong>GitLab</strong></li>
              <li><strong>Local</strong></li>
            </ul>
          </div>

          <div className="rate-limit-info">
            <small>{t('sidebar.rateLimitInfo')}</small>
          </div>
        </div>
      )}
    </div>
  );
};
