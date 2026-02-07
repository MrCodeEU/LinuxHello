#!/bin/bash
# Test which camera and resolution combo works best

echo "=== Camera Device Information ==="
echo ""
echo "RGB Camera (video0):"
echo "  Resolution: 1920x1080 MJPEG"
echo "  Use case: General purpose, color images"
echo ""
echo "IR Camera (video2):"
echo "  Resolution: 640x360 GREY (infrared)"
echo "  Use case: Low light, harder to spoof"
echo ""

echo "=== Current Configuration ==="
grep -A 20 "^camera:" /etc/linuxhello/linuxhello.conf | head -15
echo ""

echo "=== Recommendation ==="
echo "For best face authentication:"
echo "  1. Use IR camera (video2) if you have IR illumination"
echo "  2. Use RGB camera (video0) if IR doesn't work well"
echo ""
echo "Current config uses: $(grep 'device:' /etc/linuxhello/linuxhello.conf | head -1 | awk '{print $2}')"
echo "Resolution in config: $(grep 'width:' /etc/linuxhello/linuxhello.conf | head -1 | awk '{print $2}')x$(grep 'height:' /etc/linuxhello/linuxhello.conf | head -1 | awk '{print $2}')"
