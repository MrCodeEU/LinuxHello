export namespace auth {
	
	export class DebugBoundingBox {
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new DebugBoundingBox(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.confidence = source["confidence"];
	    }
	}

}

export namespace config {
	
	export class AuthConfig {
	    max_attempts: number;
	    lockout_duration: number;
	    session_timeout: number;
	    fallback_enabled: boolean;
	    continuous_auth: boolean;
	    security_level: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.max_attempts = source["max_attempts"];
	        this.lockout_duration = source["lockout_duration"];
	        this.session_timeout = source["session_timeout"];
	        this.fallback_enabled = source["fallback_enabled"];
	        this.continuous_auth = source["continuous_auth"];
	        this.security_level = source["security_level"];
	    }
	}
	export class CameraConfig {
	    device: string;
	    ir_device: string;
	    depth_device: string;
	    width: number;
	    height: number;
	    fps: number;
	    pixel_format: string;
	    use_realsense: boolean;
	    auto_exposure: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CameraConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.ir_device = source["ir_device"];
	        this.depth_device = source["depth_device"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.fps = source["fps"];
	        this.pixel_format = source["pixel_format"];
	        this.use_realsense = source["use_realsense"];
	        this.auto_exposure = source["auto_exposure"];
	    }
	}
	export class ChallengeConfig {
	    enabled: boolean;
	    challenge_types: string[];
	    timeout_seconds: number;
	    required_success: number;
	
	    static createFrom(source: any = {}) {
	        return new ChallengeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.challenge_types = source["challenge_types"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.required_success = source["required_success"];
	    }
	}
	export class LoggingConfig {
	    level: string;
	    file: string;
	    max_size: number;
	    max_backups: number;
	    max_age: number;
	
	    static createFrom(source: any = {}) {
	        return new LoggingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.file = source["file"];
	        this.max_size = source["max_size"];
	        this.max_backups = source["max_backups"];
	        this.max_age = source["max_age"];
	    }
	}
	export class StorageConfig {
	    data_dir: string;
	    database_path: string;
	    max_users: number;
	    backup_enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StorageConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data_dir = source["data_dir"];
	        this.database_path = source["database_path"];
	        this.max_users = source["max_users"];
	        this.backup_enabled = source["backup_enabled"];
	    }
	}
	export class LockoutConfig {
	    enabled: boolean;
	    max_failures: number;
	    lockout_duration: number;
	    progressive_lockout: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LockoutConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.max_failures = source["max_failures"];
	        this.lockout_duration = source["lockout_duration"];
	        this.progressive_lockout = source["progressive_lockout"];
	    }
	}
	export class LivenessConfig {
	    enabled: boolean;
	    model_path: string;
	    depth_threshold: number;
	    confidence_threshold: number;
	    use_depth_camera: boolean;
	    use_ir_analysis: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LivenessConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.model_path = source["model_path"];
	        this.depth_threshold = source["depth_threshold"];
	        this.confidence_threshold = source["confidence_threshold"];
	        this.use_depth_camera = source["use_depth_camera"];
	        this.use_ir_analysis = source["use_ir_analysis"];
	    }
	}
	export class RecognitionConfig {
	    model_path: string;
	    input_size: number;
	    embedding_size: number;
	    similarity_threshold: number;
	    enrollment_samples: number;
	
	    static createFrom(source: any = {}) {
	        return new RecognitionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model_path = source["model_path"];
	        this.input_size = source["input_size"];
	        this.embedding_size = source["embedding_size"];
	        this.similarity_threshold = source["similarity_threshold"];
	        this.enrollment_samples = source["enrollment_samples"];
	    }
	}
	export class DetectionConfig {
	    model_path: string;
	    confidence: number;
	    nms_threshold: number;
	    input_size: number;
	    max_detections: number;
	
	    static createFrom(source: any = {}) {
	        return new DetectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model_path = source["model_path"];
	        this.confidence = source["confidence"];
	        this.nms_threshold = source["nms_threshold"];
	        this.input_size = source["input_size"];
	        this.max_detections = source["max_detections"];
	    }
	}
	export class InferenceConfig {
	    address: string;
	    timeout: number;
	
	    static createFrom(source: any = {}) {
	        return new InferenceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.timeout = source["timeout"];
	    }
	}
	export class Config {
	    inference: InferenceConfig;
	    camera: CameraConfig;
	    detection: DetectionConfig;
	    recognition: RecognitionConfig;
	    liveness: LivenessConfig;
	    challenge: ChallengeConfig;
	    lockout: LockoutConfig;
	    auth: AuthConfig;
	    storage: StorageConfig;
	    logging: LoggingConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inference = this.convertValues(source["inference"], InferenceConfig);
	        this.camera = this.convertValues(source["camera"], CameraConfig);
	        this.detection = this.convertValues(source["detection"], DetectionConfig);
	        this.recognition = this.convertValues(source["recognition"], RecognitionConfig);
	        this.liveness = this.convertValues(source["liveness"], LivenessConfig);
	        this.challenge = this.convertValues(source["challenge"], ChallengeConfig);
	        this.lockout = this.convertValues(source["lockout"], LockoutConfig);
	        this.auth = this.convertValues(source["auth"], AuthConfig);
	        this.storage = this.convertValues(source["storage"], StorageConfig);
	        this.logging = this.convertValues(source["logging"], LoggingConfig);
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

export namespace main {
	
	export class AuthTestResult {
	    success: boolean;
	    error?: string;
	    user?: string;
	    confidence?: number;
	    processing_time?: string;
	    liveness_passed: boolean;
	    challenge_description?: string;
	    image_data?: string;
	    image_width?: number;
	    image_height?: number;
	    bounding_boxes?: auth.DebugBoundingBox[];
	    faces_detected: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.user = source["user"];
	        this.confidence = source["confidence"];
	        this.processing_time = source["processing_time"];
	        this.liveness_passed = source["liveness_passed"];
	        this.challenge_description = source["challenge_description"];
	        this.image_data = source["image_data"];
	        this.image_width = source["image_width"];
	        this.image_height = source["image_height"];
	        this.bounding_boxes = this.convertValues(source["bounding_boxes"], auth.DebugBoundingBox);
	        this.faces_detected = source["faces_detected"];
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
	export class EnrollmentStatus {
	    is_enrolling: boolean;
	    username: string;
	    progress: number;
	    total: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new EnrollmentStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_enrolling = source["is_enrolling"];
	        this.username = source["username"];
	        this.progress = source["progress"];
	        this.total = source["total"];
	        this.message = source["message"];
	    }
	}
	export class LogEntry {
	    timestamp: string;
	    level: string;
	    message: string;
	    component?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.message = source["message"];
	        this.component = source["component"];
	    }
	}
	export class ModelInfo {
	    name: string;
	    path: string;
	    exists: boolean;
	    size: number;
	    required: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.size = source["size"];
	        this.required = source["required"];
	    }
	}
	export class ModelStatus {
	    detectionModel: ModelInfo;
	    recognitionModel: ModelInfo;
	    allModelsPresent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detectionModel = this.convertValues(source["detectionModel"], ModelInfo);
	        this.recognitionModel = this.convertValues(source["recognitionModel"], ModelInfo);
	        this.allModelsPresent = source["allModelsPresent"];
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
	export class PAMServiceStatus {
	    id: string;
	    name: string;
	    pamFile: string;
	    status: string;
	    modulePath: string;
	
	    static createFrom(source: any = {}) {
	        return new PAMServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.pamFile = source["pamFile"];
	        this.status = source["status"];
	        this.modulePath = source["modulePath"];
	    }
	}
	export class ServiceInfo {
	    status: string;
	    enabled: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.enabled = source["enabled"];
	    }
	}
	export class UserResponse {
	    username: string;
	    samples: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UserResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.samples = source["samples"];
	        this.active = source["active"];
	    }
	}

}

