import 'json.dart';

/// Public category tree er node (`GET /api/categories`); naam age-i locale e resolve.
class Category {
  /// Id.
  final int id;

  /// URL er slug.
  final String slug, name;

  /// Chobi (storage key).
  final String? imageKey;

  /// Child category.
  final List<Category> children;

  /// Category.
  const Category(this.id, this.slug, this.name, this.imageKey, this.children);

  /// JSON theke.
  factory Category.fromJson(Map<String, dynamic> j) => Category(
    field(j, 'id'),
    field(j, 'slug'),
    field(j, 'name'),
    field(j, 'image_key'),
    objects(j, 'children', Category.fromJson),
  );
}

/// Product list er card (`GET /api/products`). Dam poisha (`int`).
class ProductCard {
  /// Id.
  final int id;

  /// Slug, naam (locale e), unit.
  final String slug, name, unit;

  /// Kg er dam (8.5).
  final bool variableWeight;

  /// Sobcheye kom active variant er dam (poisha).
  final int price;

  /// Ager dam (poisha).
  final int? compareAtPrice;

  /// Kono variant e stock ache.
  final bool inStock;

  /// Gor rating (0 = review nai).
  final double rating;

  /// Review er shonkhya.
  final int reviewCount;

  /// Prothom chobi (storage key).
  final String? image;

  /// ProductCard.
  const ProductCard(
    this.id,
    this.slug,
    this.name,
    this.unit,
    this.variableWeight,
    this.price,
    this.compareAtPrice,
    this.inStock,
    this.rating,
    this.reviewCount,
    this.image,
  );

  /// JSON theke.
  factory ProductCard.fromJson(Map<String, dynamic> j) => ProductCard(
    field(j, 'id'),
    field(j, 'slug'),
    field(j, 'name'),
    field(j, 'unit'),
    field(j, 'variable_weight'),
    field(j, 'price'),
    field(j, 'compare_at_price'),
    field(j, 'in_stock'),
    number(j, 'rating'),
    field(j, 'review_count'),
    field(j, 'image'),
  );
}

/// Product er variant (detail e).
class Variant {
  /// Id.
  final int id;

  /// SKU.
  final String sku;

  /// Option (jemon weight → "৫ কেজি"), locale e.
  final Map<String, String> options;

  /// Dam (poisha; variable_weight hole 1 kg er).
  final int price;

  /// Ager dam, ek unit er ojon (gram).
  final int? compareAtPrice, nominalWeightG;

  /// Stock ache.
  final bool inStock;

  /// Variant.
  const Variant(
    this.id,
    this.sku,
    this.options,
    this.price,
    this.compareAtPrice,
    this.nominalWeightG,
    this.inStock,
  );

  /// JSON theke.
  factory Variant.fromJson(Map<String, dynamic> j) => Variant(
    field(j, 'id'),
    field(j, 'sku'),
    field<Map<String, dynamic>>(j, 'options').cast<String, String>(),
    field(j, 'price'),
    field(j, 'compare_at_price'),
    field(j, 'nominal_weight_g'),
    field(j, 'in_stock'),
  );
}

/// Product er chobi.
class ProductImage {
  /// Storage key, alt (locale e).
  final String storageKey, alt;

  /// Pixel.
  final int width, height;

  /// ProductImage.
  const ProductImage(this.storageKey, this.alt, this.width, this.height);

  /// JSON theke.
  factory ProductImage.fromJson(Map<String, dynamic> j) => ProductImage(
    field(j, 'storage_key'),
    field(j, 'alt'),
    field(j, 'width'),
    field(j, 'height'),
  );
}

/// Product detail (`GET /api/products/{slug}`).
class Product {
  /// Id.
  final int id;

  /// Slug, naam, description (locale e), unit.
  final String slug, name, description, unit;

  /// Kg er dam (8.5).
  final bool variableWeight;

  /// Category er slug.
  final String? category;

  /// Origin, certification ityadi; SEO (locale e). Vendor er lekha: HTML e escape kore boshao.
  final Map<String, dynamic> attributes, seo;

  /// Gor rating.
  final double rating;

  /// Review er shonkhya.
  final int reviewCount;

  /// Active variant (sort_order kram e).
  final List<Variant> variants;

  /// Chobi (sort_order kram e).
  final List<ProductImage> images;

  /// Product.
  const Product(
    this.id,
    this.slug,
    this.name,
    this.description,
    this.unit,
    this.variableWeight,
    this.category,
    this.attributes,
    this.seo,
    this.rating,
    this.reviewCount,
    this.variants,
    this.images,
  );

  /// JSON theke.
  factory Product.fromJson(Map<String, dynamic> j) => Product(
    field(j, 'id'),
    field(j, 'slug'),
    field(j, 'name'),
    field(j, 'description'),
    field(j, 'unit'),
    field(j, 'variable_weight'),
    field(j, 'category'),
    field(j, 'attributes'),
    field(j, 'seo'),
    number(j, 'rating'),
    field(j, 'review_count'),
    objects(j, 'variants', Variant.fromJson),
    objects(j, 'images', ProductImage.fromJson),
  );
}
