export namespace main {
	
	export class AppStatus {
	    connected: boolean;
	    reconnecting: boolean;
	    server: string;
	    room: string;
	    virtualIP: string;
	    peerCount: number;
	    isOwner: boolean;
	    phase: string;
	
	    static createFrom(source: any = {}) {
	        return new AppStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.reconnecting = source["reconnecting"];
	        this.server = source["server"];
	        this.room = source["room"];
	        this.virtualIP = source["virtualIP"];
	        this.peerCount = source["peerCount"];
	        this.isOwner = source["isOwner"];
	        this.phase = source["phase"];
	    }
	}
	export class ClientConfig {
	    serverAddr: string;
	    nickname: string;
	    autoConnect: boolean;
	    autoJoinLastRoom: boolean;
	    lastRoomName: string;
	    lastRoomLocked: boolean;
	    startWithWindows: boolean;
	    connectionMode: string;
	    p2pOnly?: boolean;
	    mtu: number;
	    dnsServer: string;
	    socks5Proxy: string;
	    stunServer: string;
	    fontSize?: number;
	    uiScale: number;
	    theme: string;
	    lang: string;
	    serverToken: string;
	
	    static createFrom(source: any = {}) {
	        return new ClientConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverAddr = source["serverAddr"];
	        this.nickname = source["nickname"];
	        this.autoConnect = source["autoConnect"];
	        this.autoJoinLastRoom = source["autoJoinLastRoom"];
	        this.lastRoomName = source["lastRoomName"];
	        this.lastRoomLocked = source["lastRoomLocked"];
	        this.startWithWindows = source["startWithWindows"];
	        this.connectionMode = source["connectionMode"];
	        this.p2pOnly = source["p2pOnly"];
	        this.mtu = source["mtu"];
	        this.dnsServer = source["dnsServer"];
	        this.socks5Proxy = source["socks5Proxy"];
	        this.stunServer = source["stunServer"];
	        this.fontSize = source["fontSize"];
	        this.uiScale = source["uiScale"];
	        this.theme = source["theme"];
	        this.lang = source["lang"];
	        this.serverToken = source["serverToken"];
	    }
	}
	export class PeerInfo {
	    id: string;
	    nickname: string;
	    virtualIP: string;
	    connected: boolean;
	    ping: number;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new PeerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nickname = source["nickname"];
	        this.virtualIP = source["virtualIP"];
	        this.connected = source["connected"];
	        this.ping = source["ping"];
	        this.path = source["path"];
	    }
	}
	export class SavedRoomEntry {
	    name: string;
	    password?: string;
	    server: string;
	    locked?: boolean;
	    owner_token?: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedRoomEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.password = source["password"];
	        this.server = source["server"];
	        this.locked = source["locked"];
	        this.owner_token = source["owner_token"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    available: boolean;
	    notes: string;
	    assetURL: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.available = source["available"];
	        this.notes = source["notes"];
	        this.assetURL = source["assetURL"];
	    }
	}

}

