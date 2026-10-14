#!/usr/bin/env python3
"""Regression boundaries for Modern's deliberately non-pixel-perfect audit."""
import unittest
from PIL import Image, ImageDraw
from modern_presentation_compare import accuracy_mask


class ModernAccuracyTests(unittest.TestCase):
    def fixture(self):
        img = Image.new('RGB',(40,30))
        draw = ImageDraw.Draw(img)
        draw.rectangle((10,5,29,24),fill=(120,80,40))
        draw.rectangle((15,10,24,19),fill=(200,160,100))
        return img

    def test_single_texel_sampling_shift_is_allowed(self):
        a = self.fixture(); b = Image.new('RGB',a.size);b.paste(a,(1,1))
        self.assertIsNone(accuracy_mask(a,b).getbbox())

    def test_wrong_anchor_is_rejected(self):
        a = self.fixture(); b = Image.new('RGB',a.size);b.paste(a,(0,8))
        self.assertIsNotNone(accuracy_mask(a,b).getbbox())

    def test_wrong_brightness_is_rejected(self):
        a = self.fixture();b = a.point(lambda x:x//2)
        self.assertIsNotNone(accuracy_mask(a,b).getbbox())

    def test_missing_weapon_is_rejected(self):
        a = self.fixture()
        self.assertIsNotNone(accuracy_mask(a,Image.new('RGB',a.size)).getbbox())

    def test_missing_internal_artwork_is_rejected(self):
        a = self.fixture();b = a.copy();ImageDraw.Draw(b).rectangle((15,10,24,19),fill=(120,80,40))
        self.assertIsNotNone(accuracy_mask(a,b).getbbox())


if __name__=='__main__':
    unittest.main()
