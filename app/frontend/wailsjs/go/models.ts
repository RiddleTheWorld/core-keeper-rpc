export namespace main {
	
	export class Lines {
	    details: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new Lines(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.details = source["details"];
	        this.state = source["state"];
	    }
	}
	export class Config {
	    enabled: boolean;
	    showTimer: boolean;
	    showImage: boolean;
	    imageText: string;
	    updateSecs: number;
	    situations: Record<string, Lines>;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.showTimer = source["showTimer"];
	        this.showImage = source["showImage"];
	        this.imageText = source["imageText"];
	        this.updateSecs = source["updateSecs"];
	        this.situations = this.convertValues(source["situations"], Lines, true);
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
	
	export class LiveState {
	    game: string;
	    discord: string;
	    situation: string;
	    details: string;
	    state: string;
	    tokens: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new LiveState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = source["game"];
	        this.discord = source["discord"];
	        this.situation = source["situation"];
	        this.details = source["details"];
	        this.state = source["state"];
	        this.tokens = source["tokens"];
	    }
	}

}

