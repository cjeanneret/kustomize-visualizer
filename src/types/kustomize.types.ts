export interface KustomizeNode {
  id: string;
  path: string;
  type: 'resource' | 'component';  // Seulement 2 types !
  kustomizationContent: KustomizationYaml;
  isRemote: boolean;
  remoteUrl?: string;
  loaded: boolean;
}

export interface DependencyEdge {
  id: string;
  source: string;
  target: string;
  type: 'resource' | 'component';  // Seulement 2 types !
  label?: string;
}

export interface KustomizeGraph {
  nodes: Map<string, KustomizeNode>;
  edges: DependencyEdge[];
  rootPath: string;
}

export interface KustomizationYaml {
  apiVersion?: string;
  kind?: string;
  resources?: string[];
  bases?: string[];  // Déprécié, traité comme resources
  components?: string[];
  patches?: any[];
  patchesStrategicMerge?: string[];
  configMapGenerator?: any[];
  secretGenerator?: any[];
}
