export namespace discover {
	
	export class Gateway {
	    ip: string;
	    host: string;
	    name: string;
	    model: string;
	    serial: string;
	
	    static createFrom(source: any = {}) {
	        return new Gateway(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.host = source["host"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.serial = source["serial"];
	    }
	}

}

export namespace main {
	
	export class ConnectResult {
	    gateway: string;
	    meters: zgw.MeterInfo[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gateway = source["gateway"];
	        this.meters = this.convertValues(source["meters"], zgw.MeterInfo);
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
	export class MeterSelection {
	    busAddress: number;
	    registers: number[];
	
	    static createFrom(source: any = {}) {
	        return new MeterSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.busAddress = source["busAddress"];
	        this.registers = source["registers"];
	    }
	}
	export class ExportRequest {
	    meters: MeterSelection[];
	    timeFrames: number[];
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.meters = this.convertValues(source["meters"], MeterSelection);
	        this.timeFrames = source["timeFrames"];
	        this.outputDir = source["outputDir"];
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
	
	export class Settings {
	    host: string;
	    password: string;
	    remember: boolean;
	    outputDir: string;
	    timeFrames: number[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.password = source["password"];
	        this.remember = source["remember"];
	        this.outputDir = source["outputDir"];
	        this.timeFrames = source["timeFrames"];
	    }
	}

}

export namespace zgw {
	
	export class RecordedRegister {
	    number: number;
	    name: string;
	    unit: string;
	
	    static createFrom(source: any = {}) {
	        return new RecordedRegister(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.name = source["name"];
	        this.unit = source["unit"];
	    }
	}
	export class MeterInfo {
	    busAddress: number;
	    name: string;
	    typeName: string;
	    registers: RecordedRegister[];
	
	    static createFrom(source: any = {}) {
	        return new MeterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.busAddress = source["busAddress"];
	        this.name = source["name"];
	        this.typeName = source["typeName"];
	        this.registers = this.convertValues(source["registers"], RecordedRegister);
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

