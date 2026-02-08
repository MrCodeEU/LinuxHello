# Testing the New Model Download Feature

## Quick Testing Guide

### 1. Uninstall Previous Version

First, remove the old installation:

```bash
# Stop any running services
sudo systemctl stop linuxhello-inference
sudo pkill -f "python3.*inference_service"

# Uninstall via Makefile
sudo make uninstall

# (Optional) Clean all data/config if you want a fresh start
sudo rm -rf /etc/linuxhello /opt/linuxhello /var/lib/linuxhello
```

### 2. Build the New Version

```bash
# Setup dependencies (if not already done)
make setup-python
make deps

# Build the app (don't download models yet!)
cd frontend && npm install && npm run build && cd ..
go build -ldflags="-s -w" -tags desktop,production -o bin/linuxhello .

# Build PAM module
CGO_ENABLED=1 go build -ldflags="-s -w" -buildmode=c-shared -o bin/pam_linuxhello.so ./pkg/pam
```

### 3. Test Without Installing (Recommended)

Test the app locally without system-wide installation:

```bash
# Start the inference service in background
make start-service-bg

# Run the app with sudo (required for camera/PAM access)
sudo ./bin/linuxhello

# The model download modal should appear automatically!
```

When you launch, you should see:
- ✅ Model download modal appears on first launch
- ✅ Shows status of both models (det_10g.onnx, arcface_r50.onnx)
- ✅ "Download Models" button works
- ✅ Models download to `./models/` directory
- ✅ Modal closes automatically when models are present

### 4. Test Full Installation (Optional)

If you want to test the full system installation:

```bash
# Create models directory but don't populate it
mkdir -p models

# Install system-wide (without models)
sudo make install

# Start the service
sudo systemctl start linuxhello-inference

# Launch GUI
sudo linuxhello

# Modal should appear and download to /opt/linuxhello/models/
```

### 5. Clean Up After Testing

```bash
# Stop services
sudo systemctl stop linuxhello-inference
make stop-service

# If you want to test again from scratch
rm -rf models/*.onnx
rm -rf data/dev/*.db

# The next launch will show the modal again
```

## Testing Checklist

- [ ] Modal appears on first launch when models are missing
- [ ] Modal shows both model names and status
- [ ] "Download Models" button downloads both files
- [ ] Progress indication shows while downloading
- [ ] Modal closes after successful download
- [ ] App works normally after models are downloaded
- [ ] Subsequent launches don't show modal (models already present)
- [ ] Error messages appear if download fails

## Development Mode (Wails)

For rapid frontend testing with hot reload:

```bash
# Terminal 1: Start inference service
make start-service-bg

# Terminal 2: Run Wails dev mode
wails dev

# The dev version will check for models in ./models/
# Delete models/*.onnx to test the modal again
```

## Troubleshooting

**Modal doesn't appear:**
- Check browser console (F12) for errors
- Verify `CheckModels()` is being called in App.tsx useEffect
- Check if models already exist in `./models/` or `/opt/linuxhello/models/`

**Download fails:**
- Check network connection
- Verify HuggingFace URLs are accessible
- Check write permissions on models directory
- Look at logs: `tail -f logs/inference.log`

**Models not detected after download:**
- Check file paths match what `CheckModels()` expects
- Verify files are complete (det_10g.onnx ~17MB, arcface_r50.onnx ~170MB)
- Check permissions: `ls -la models/`

## Expected Behavior

1. **First Launch (no models):**
   - App starts
   - Modal appears immediately
   - Shows "Download Models" button
   - Click → Downloads ~200MB from HuggingFace
   - Modal closes when complete

2. **Subsequent Launches (models present):**
   - App starts
   - Quick model check happens
   - No modal appears
   - App works normally

3. **Partial Models:**
   - Modal appears showing which models are missing
   - Download button gets only missing files
