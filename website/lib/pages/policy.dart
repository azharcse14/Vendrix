import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';

import '../components/plain_text.dart';
import '../components/site.dart';
import 'status.dart';
import '../seo/meta.dart';

/// Policy er kind (10.1); footer er link o eta theke.
const policyKinds = ['terms', 'return', 'refund', 'delivery', 'privacy'];

/// Policy (`/policy/{kind}`, 10.1, v1.13): API er lekha (shop er, na hole Bangla placeholder) [plainText] e.
class PolicyPage extends AsyncStatelessComponent {
  /// `terms`, `return`, `refund`, `delivery`, `privacy`.
  final String kind;

  /// PolicyPage.
  const PolicyPage(this.kind, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final String body;
    try {
      body = await site.catalog.policy(kind);
    } catch (e) {
      return pageError(context, e);
    }
    return article(classes: 'mx-auto max-w-screen-md px-4 py-8', [
      SeoHead(title: site.t('policy.$kind'), description: body),
      h1(classes: 'mb-4', [.text(site.t('policy.$kind'))]),
      ...plainText(body),
    ]);
  }
}
