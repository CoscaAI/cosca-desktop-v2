export namespace main {
	
	export class Approval {
	    id: string;
	    request: string;
	    model: string;
	    scope: string;
	    risk: string;
	    status: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Approval(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.request = source["request"];
	        this.model = source["model"];
	        this.scope = source["scope"];
	        this.risk = source["risk"];
	        this.status = source["status"];
	        this.created_at = source["created_at"];
	    }
	}
	export class ChatMessage {
	    role: string;
	    content: string;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.at = source["at"];
	    }
	}
	export class FileNode {
	    name: string;
	    path: string;
	    kind: string;
	    is_dir: boolean;
	    is_symlink: boolean;
	    ext: string;
	    language: string;
	    size: number;
	    modified_at: string;
	    children?: FileNode[];
	    loaded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.is_dir = source["is_dir"];
	        this.is_symlink = source["is_symlink"];
	        this.ext = source["ext"];
	        this.language = source["language"];
	        this.size = source["size"];
	        this.modified_at = source["modified_at"];
	        this.children = this.convertValues(source["children"], FileNode);
	        this.loaded = source["loaded"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Project {
	    name: string;
	    root: string;
	    initialized: boolean;
	    has_cosca: boolean;
	    inited_at: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.root = source["root"];
	        this.initialized = source["initialized"];
	        this.has_cosca = source["has_cosca"];
	        this.inited_at = source["inited_at"];
	        this.error = source["error"];
	    }
	}
	export class RuntimeEvent {
	    type: string;
	    raw_type: string;
	    seq: number;
	    content: string;
	    session_id: string;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.raw_type = source["raw_type"];
	        this.seq = source["seq"];
	        this.content = source["content"];
	        this.session_id = source["session_id"];
	        this.at = source["at"];
	    }
	}
	export class TreeEntry {
	    name: string;
	    path: string;
	    kind: string;
	    children?: TreeEntry[];
	
	    static createFrom(source: any = {}) {
	        return new TreeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.children = this.convertValues(source["children"], TreeEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

