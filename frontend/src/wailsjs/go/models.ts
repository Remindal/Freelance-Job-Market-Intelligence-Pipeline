export namespace desktop {
	
	export class Dimension {
	    score: number;
	    analysis: string;
	
	    static createFrom(source: any = {}) {
	        return new Dimension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.analysis = source["analysis"];
	    }
	}
	export class AnalysisReport {
	    overall: number;
	    verdict: string;
	    dimensions: Record<string, Dimension>;
	    pitch_angle: string;
	    risks: string[];
	
	    static createFrom(source: any = {}) {
	        return new AnalysisReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall = source["overall"];
	        this.verdict = source["verdict"];
	        this.dimensions = this.convertValues(source["dimensions"], Dimension, true);
	        this.pitch_angle = source["pitch_angle"];
	        this.risks = source["risks"];
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
	
	export class JobDetail {
	    id: number;
	    url: string;
	    title: string;
	    description: string;
	    budget: string;
	    skills: string[];
	    score: number;
	    reason: string;
	    tags: string[];
	    status: string;
	    // Go type: time
	    fetched_at: any;
	    // Go type: time
	    scored_at?: any;
	    payment_verified?: boolean;
	    client_spent_usd?: number;
	    client_rating?: number;
	    // Go type: time
	    posted_at?: any;
	    proposals_bucket: string;
	    // Go type: time
	    last_viewed_at?: any;
	    interviewing?: number;
	    invites_sent?: number;
	    analysis?: AnalysisReport;
	
	    static createFrom(source: any = {}) {
	        return new JobDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.budget = source["budget"];
	        this.skills = source["skills"];
	        this.score = source["score"];
	        this.reason = source["reason"];
	        this.tags = source["tags"];
	        this.status = source["status"];
	        this.fetched_at = this.convertValues(source["fetched_at"], null);
	        this.scored_at = this.convertValues(source["scored_at"], null);
	        this.payment_verified = source["payment_verified"];
	        this.client_spent_usd = source["client_spent_usd"];
	        this.client_rating = source["client_rating"];
	        this.posted_at = this.convertValues(source["posted_at"], null);
	        this.proposals_bucket = source["proposals_bucket"];
	        this.last_viewed_at = this.convertValues(source["last_viewed_at"], null);
	        this.interviewing = source["interviewing"];
	        this.invites_sent = source["invites_sent"];
	        this.analysis = this.convertValues(source["analysis"], AnalysisReport);
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
	export class JobListResult {
	    items: domain.Job[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new JobListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], domain.Job);
	        this.total = source["total"];
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
	export class ListFilter {
	    status: string;
	    min_score: number;
	    keyword: string;
	    tag: string;
	    page: number;
	    page_size: number;
	    sort: string;
	
	    static createFrom(source: any = {}) {
	        return new ListFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.min_score = source["min_score"];
	        this.keyword = source["keyword"];
	        this.tag = source["tag"];
	        this.page = source["page"];
	        this.page_size = source["page_size"];
	        this.sort = source["sort"];
	    }
	}
	export class ProgressView {
	    active: boolean;
	    stage: string;
	    feed: string;
	    feed_index: number;
	    feed_total: number;
	    feed_jobs: number;
	    score_done: number;
	    score_total: number;
	    new: number;
	    fetched: number;
	
	    static createFrom(source: any = {}) {
	        return new ProgressView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.stage = source["stage"];
	        this.feed = source["feed"];
	        this.feed_index = source["feed_index"];
	        this.feed_total = source["feed_total"];
	        this.feed_jobs = source["feed_jobs"];
	        this.score_done = source["score_done"];
	        this.score_total = source["score_total"];
	        this.new = source["new"];
	        this.fetched = source["fetched"];
	    }
	}
	export class RunResult {
	    success: boolean;
	    fetched: number;
	    new: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.fetched = source["fetched"];
	        this.new = source["new"];
	        this.message = source["message"];
	    }
	}

}

export namespace domain {
	
	export class Job {
	    id: number;
	    url: string;
	    title: string;
	    description: string;
	    budget: string;
	    skills: string[];
	    score: number;
	    reason: string;
	    tags: string[];
	    status: string;
	    // Go type: time
	    fetched_at: any;
	    // Go type: time
	    scored_at?: any;
	    payment_verified?: boolean;
	    client_spent_usd?: number;
	    client_rating?: number;
	    // Go type: time
	    posted_at?: any;
	    proposals_bucket: string;
	    // Go type: time
	    last_viewed_at?: any;
	    interviewing?: number;
	    invites_sent?: number;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.budget = source["budget"];
	        this.skills = source["skills"];
	        this.score = source["score"];
	        this.reason = source["reason"];
	        this.tags = source["tags"];
	        this.status = source["status"];
	        this.fetched_at = this.convertValues(source["fetched_at"], null);
	        this.scored_at = this.convertValues(source["scored_at"], null);
	        this.payment_verified = source["payment_verified"];
	        this.client_spent_usd = source["client_spent_usd"];
	        this.client_rating = source["client_rating"];
	        this.posted_at = this.convertValues(source["posted_at"], null);
	        this.proposals_bucket = source["proposals_bucket"];
	        this.last_viewed_at = this.convertValues(source["last_viewed_at"], null);
	        this.interviewing = source["interviewing"];
	        this.invites_sent = source["invites_sent"];
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

export namespace store {
	
	export class DailyCount {
	    date: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.count = source["count"];
	    }
	}
	export class ScoreBucket {
	    label: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ScoreBucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.count = source["count"];
	    }
	}
	export class Stats {
	    today_new: number;
	    high_score_pending: number;
	    want_count: number;
	    proposed_count: number;
	    daily_new: DailyCount[];
	    score_distribution: ScoreBucket[];
	    status_counts: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.today_new = source["today_new"];
	        this.high_score_pending = source["high_score_pending"];
	        this.want_count = source["want_count"];
	        this.proposed_count = source["proposed_count"];
	        this.daily_new = this.convertValues(source["daily_new"], DailyCount);
	        this.score_distribution = this.convertValues(source["score_distribution"], ScoreBucket);
	        this.status_counts = source["status_counts"];
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

